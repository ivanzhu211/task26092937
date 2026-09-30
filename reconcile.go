package reconcile

import (
	"container/heap"
	"sort"
)

// Reconcile 返回账单行到收货批次的数量分配建议。
//
// 建模为最小费用最大流：源->账单行（容量=账单数量），账单行->批次
// （仅当 SKU 相同且批次采购单行ID在允许集合内，费用=单价绝对差），
// 批次->汇（容量=批次净收货量）。所有费用非负，SSP 保证先取得最大流，
// 并在所有最大流中取得最小费用，即两级目标：先最大化总匹配数量，
// 再最小化总绝对价差。
//
// 不消耗或修改输入账本，可重复调用。内部按 ID 排序建图，
// 输入顺序不影响两个目标值，输出分配按 (LineID, BatchID) 排序。
func Reconcile(ledger Ledger, bills []BillLine) Suggestion {
	batches := make([]Batch, len(ledger.Batches))
	copy(batches, ledger.Batches)
	sort.Slice(batches, func(i, j int) bool { return batches[i].BatchID < batches[j].BatchID })

	lines := make([]BillLine, len(bills))
	copy(lines, bills)
	sort.Slice(lines, func(i, j int) bool { return lines[i].LineID < lines[j].LineID })

	numBills := len(lines)
	numBatches := len(batches)
	source := 0
	billBase := 1
	batchBase := billBase + numBills
	sink := batchBase + numBatches

	g := newGraph(sink + 1)

	billNode := make([]int, numBills)
	for i, bl := range lines {
		billNode[i] = billBase + i
		if bl.Qty > 0 {
			g.addEdge(source, billNode[i], bl.Qty, 0)
		}
	}
	batchNode := make([]int, numBatches)
	for j, bt := range batches {
		batchNode[j] = batchBase + j
		if bt.NetQty > 0 {
			g.addEdge(batchNode[j], sink, bt.NetQty, 0)
		}
	}

	type matchEdge struct{ billIdx, batchIdx, edgeIdx int }
	var matchEdges []matchEdge
	for i, bl := range lines {
		if bl.Qty <= 0 || len(bl.POLineIDs) == 0 {
			continue
		}
		allowed := make(map[string]struct{}, len(bl.POLineIDs))
		for _, id := range bl.POLineIDs {
			allowed[id] = struct{}{}
		}
		for j, bt := range batches {
			if bt.NetQty <= 0 || bt.SKU != bl.SKU {
				continue
			}
			if _, ok := allowed[bt.POLineID]; !ok {
				continue
			}
			cost := bl.UnitPrice - bt.UnitPrice
			if cost < 0 {
				cost = -cost
			}
			cap := bl.Qty
			if bt.NetQty < cap {
				cap = bt.NetQty
			}
			edgeIdx := g.addEdge(billNode[i], batchNode[j], cap, cost)
			matchEdges = append(matchEdges, matchEdge{i, j, edgeIdx})
		}
	}

	g.minCostMaxFlow(source, sink)

	sg := Suggestion{}
	for _, me := range matchEdges {
		e := g.edges[me.edgeIdx]
		orig := e.cap + g.edges[me.edgeIdx^1].cap // 剩余容量 + 反向流量 = 原始容量
		flow := orig - e.cap
		if flow <= 0 {
			continue
		}
		sg.Allocations = append(sg.Allocations, Allocation{
			LineID:  lines[me.billIdx].LineID,
			BatchID: batches[me.batchIdx].BatchID,
			Qty:     flow,
		})
		sg.MatchedQty += flow
		sg.TotalAbsDiff += flow * g.edges[me.edgeIdx].cost
	}
	return sg
}

type edge struct {
	to, rev   int
	cap, cost int64
}

type graph struct {
	edges []edge
	adj   [][]int
}

func newGraph(n int) *graph {
	return &graph{adj: make([][]int, n)}
}

func (g *graph) addEdge(from, to int, cap, cost int64) int {
	idx := len(g.edges)
	g.edges = append(g.edges, edge{to: to, rev: len(g.adj[to]), cap: cap, cost: cost})
	g.edges = append(g.edges, edge{to: from, rev: len(g.adj[from]), cap: 0, cost: -cost})
	g.adj[from] = append(g.adj[from], idx)
	g.adj[to] = append(g.adj[to], idx+1)
	return idx
}

// minCostMaxFlow 使用带势函数的连续最短路（SSP）。
// 原始费用非负，约化费用保持非负，Dijkstra 可用。
func (g *graph) minCostMaxFlow(source, sink int) {
	n := len(g.adj)
	pot := make([]int64, n)
	for {
		const inf = int64(1) << 62
		dist := make([]int64, n)
		prevV := make([]int, n)
		prevE := make([]int, n)
		for i := range dist {
			dist[i] = inf
			prevV[i] = -1
		}
		dist[source] = 0
		pq := &pqItemHeap{{v: source, d: 0}}
		for pq.Len() > 0 {
			it := heap.Pop(pq).(pqItem)
			if it.d > dist[it.v] {
				continue
			}
			for _, ei := range g.adj[it.v] {
				e := g.edges[ei]
				if e.cap <= 0 {
					continue
				}
				nd := it.d + e.cost + pot[it.v] - pot[e.to]
				if nd < dist[e.to] {
					dist[e.to] = nd
					prevV[e.to] = it.v
					prevE[e.to] = ei
					heap.Push(pq, pqItem{v: e.to, d: nd})
				}
			}
		}
		if dist[sink] == inf {
			return
		}
		for v := 0; v < n; v++ {
			if dist[v] < inf {
				pot[v] += dist[v]
			}
		}
		// 沿最短路增广 bottleneck 流量。
		aug := int64(1) << 62
		for v := sink; v != source; v = prevV[v] {
			if c := g.edges[prevE[v]].cap; c < aug {
				aug = c
			}
		}
		for v := sink; v != source; v = prevV[v] {
			ei := prevE[v]
			g.edges[ei].cap -= aug
			g.edges[ei^1].cap += aug
		}
	}
}

type pqItem struct {
	v int
	d int64
}

type pqItemHeap []pqItem

func (h pqItemHeap) Len() int            { return len(h) }
func (h pqItemHeap) Less(i, j int) bool  { return h[i].d < h[j].d }
func (h pqItemHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *pqItemHeap) Push(x interface{}) { *h = append(*h, x.(pqItem)) }
func (h *pqItemHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}
