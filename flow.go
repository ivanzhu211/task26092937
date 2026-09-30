package reconcile

import "container/heap"

// edge 为残量网络中的一条边。
type edge struct {
	to   int
	rev  int   // 反向边在 to 邻接表中的下标
	cap  int64 // 剩余容量
	cost int64 // 单位费用（非负）
}

// minCostMaxFlow 在图上求最小费用最大流。
// 所有费用必须非负。返回最大流量与该流量下的最小费用。
// graph 以邻接表表示，s 为源点，t 为汇点。
func minCostMaxFlow(graph [][]edge, s, t int) (flow int64, cost int64) {
	n := len(graph)
	potential := make([]int64, n) // 势函数，保证约化费用非负

	for {
		const inf = int64(1) << 62
		dist := make([]int64, n)
		prevNode := make([]int, n)
		prevEdge := make([]int, n)
		for i := range dist {
			dist[i] = inf
			prevNode[i] = -1
		}
		dist[s] = 0

		pq := &nodeHeap{{node: s, dist: 0}}
		for pq.Len() > 0 {
			cur := heap.Pop(pq).(nodeDist)
			if cur.dist > dist[cur.node] {
				continue
			}
			for i, e := range graph[cur.node] {
				if e.cap <= 0 {
					continue
				}
				nd := cur.dist + e.cost + potential[cur.node] - potential[e.to]
				if nd < dist[e.to] {
					dist[e.to] = nd
					prevNode[e.to] = cur.node
					prevEdge[e.to] = i
					heap.Push(pq, nodeDist{node: e.to, dist: nd})
				}
			}
		}

		if dist[t] == inf {
			return flow, cost
		}
		for v := 0; v < n; v++ {
			if dist[v] < inf {
				potential[v] += dist[v]
			}
		}

		// 沿最短路增广，瓶颈容量为本次流量。
		aug := inf
		for v := t; v != s; v = prevNode[v] {
			e := &graph[prevNode[v]][prevEdge[v]]
			if e.cap < aug {
				aug = e.cap
			}
		}
		for v := t; v != s; v = prevNode[v] {
			e := &graph[prevNode[v]][prevEdge[v]]
			e.cap -= aug
			rev := &graph[e.to][e.rev]
			rev.cap += aug
			cost += aug * e.cost
		}
		flow += aug
	}
}

// addEdge 向图中加入一条容量为 cap、单位费用为 cost 的边及其反向边。
func addEdge(graph [][]edge, from, to int, cap, cost int64) {
	graph[from] = append(graph[from], edge{to: to, rev: len(graph[to]), cap: cap, cost: cost})
	graph[to] = append(graph[to], edge{to: from, rev: len(graph[from]) - 1, cap: 0, cost: -cost})
}

type nodeDist struct {
	node int
	dist int64
}

type nodeHeap []nodeDist

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x interface{}) { *h = append(*h, x.(nodeDist)) }
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
