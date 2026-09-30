// Package reconcile 实现采购对账匹配：将供应商账单行分配到实际收货批次。
// 金额使用整数美分，数量使用整数件，不计税费。
package reconcile

// Receipt 收货记录。同一采购单行可以分批收货，各批单价可以不同。
type Receipt struct {
	BatchID       string // 批次ID
	POLineID      string // 采购单行ID
	SKU           string
	Qty           int64 // 合格接收数量（拒收数量不计入）
	UnitPriceCent int64 // 该批实际单价（美分）
}

// Return 退货记录，引用原收货批次。退货必须扣减原批次。
type Return struct {
	BatchID string // 原收货批次ID
	Qty     int64  // 退货数量
}

// BillLine 账单行。
type BillLine struct {
	LineID        string
	SKU           string
	Qty           int64
	UnitPriceCent int64
	POLineIDs     []string // 允许核销的采购单行ID集合；为空时无可匹配批次
}

// Batch 净收货账本中的一个批次，保留批次身份和实际单价。
type Batch struct {
	BatchID       string
	POLineID      string
	SKU           string
	NetQty        int64 // 合格接收数量 - 该批累计退货数量
	UnitPriceCent int64
}

// Ledger 净收货账本。
type Ledger struct {
	Batches []Batch
}

// Allocation 一笔分配：账单行 -> 收货批次。
type Allocation struct {
	LineID  string
	BatchID string
	Qty     int64
}

// Reconciliation 对账建议。
type Reconciliation struct {
	Allocations       []Allocation // 账单行到批次的数量分配
	TotalMatchedQty   int64        // 总匹配数量
	TotalAbsPriceDiff int64        // 总绝对价差（美分）
}
