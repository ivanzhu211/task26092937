package reconcile

import "sort"

// Reconcile 返回账单行到收货批次的数量分配。
//
// 建模为最小费用最大流：源点 -> 账单行（容量=账单数量，费用0），
// 账单行 -> 批次（容量无限，费用=|账单单价-批次单价|），
// 批次 -> 汇点（容量=批次净收货量，费用0）。
// 费用非负，SSP 求得的最小费用最大流即同时满足两级目标：
// 先最大化总匹配数量，再最小化总绝对价差。
//
// 不修改输入账本；同值最优方案任选，但两个目标值与输入顺序无关。
func Reconcile(ledger Ledger, bills []BillLine) Reconciliation {
	batches := ledger.Batches
	nBills := len(bills)
	nBatches := len(batches)

	// 节点编号：0=源点，1..nBills=账单行，其后为批次，最后为汇点。
	source := 0
	billNode := func(i int) int { return 1 + i }
	batchNode := func(j int) int { return 1 + nBills + j }
	sink := 1 + nBills + nBatches

	graph := make([][]edge, sink+1)

	for i, b := range bills {
		if b.Qty > 0 {
			addEdge(graph, source, billNode(i), b.Qty, 0)
		}
	}
	for j, bt := range batches {
		if bt.NetQty > 0 {
			addEdge(graph, batchNode(j), sink, bt.NetQty, 0)
		}
	}

	// 记录账单行 -> 批次边在邻接表中的位置，便于事后提取流量。
	type edgeRef struct {
		bill, batch int
		node, idx   int
	}
	var matchEdges []edgeRef
	for i, b := range bills {
		if b.Qty <= 0 {
			continue
		}
		allowed := make(map[string]bool, len(b.POLineIDs))
		for _, id := range b.POLineIDs {
			allowed[id] = true
		}
		for j, bt := range batches {
			if bt.NetQty <= 0 || bt.SKU != b.SKU || !allowed[bt.POLineID] {
				continue
			}
			cost := b.UnitPriceCent - bt.UnitPriceCent
			if cost < 0 {
				cost = -cost
			}
			from := billNode(i)
			matchEdges = append(matchEdges, edgeRef{
				bill: i, batch: j, node: from, idx: len(graph[from]),
			})
			addEdge(graph, from, batchNode(j), b.Qty, cost)
		}
	}

	matched, totalDiff := minCostMaxFlow(graph, source, sink)

	// 从账单行 -> 批次边的反向边容量提取实际流量。
	var allocs []Allocation
	for _, ref := range matchEdges {
		e := graph[ref.node][ref.idx]
		used := graph[e.to][e.rev].cap
		if used > 0 {
			allocs = append(allocs, Allocation{
				LineID:  bills[ref.bill].LineID,
				BatchID: batches[ref.batch].BatchID,
				Qty:     used,
			})
		}
	}
	sort.Slice(allocs, func(a, b int) bool {
		if allocs[a].LineID != allocs[b].LineID {
			return allocs[a].LineID < allocs[b].LineID
		}
		return allocs[a].BatchID < allocs[b].BatchID
	})

	return Reconciliation{
		Allocations:       allocs,
		TotalMatchedQty:   matched,
		TotalAbsPriceDiff: totalDiff,
	}
}
