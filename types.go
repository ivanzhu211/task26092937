// Package reconcile 提供采购对账匹配能力：将供应商账单行分配到实际收货批次，
// 输出可追溯的对账建议。金额使用整数美分，数量使用整数件，不计税费。
package reconcile

// Receipt 是一条已审核收货记录。RejectedQty 不计入 AcceptedQty。
type Receipt struct {
	BatchID     string // 收货批次ID
	POLineID    string // 采购单行ID
	SKU         string
	AcceptedQty int64 // 合格接收数量（整数件）
	UnitPrice   int64 // 该批实际单价（整数美分）
}

// Return 是一条退货记录，引用原收货批次。
// 假设引用有效，且累计退货不超过该批合格接收数量。
type Return struct {
	BatchID string // 原收货批次ID
	Qty     int64  // 退货数量
}

// BillLine 是一条供应商账单行。
type BillLine struct {
	LineID    string
	SKU       string
	Qty       int64
	UnitPrice int64    // 整数美分
	POLineIDs []string // 允许核销的采购单行ID集合；为空时无可匹配批次
}

// Batch 是账本中的净收货批次，保留批次身份与实际单价。
type Batch struct {
	BatchID   string
	POLineID  string
	SKU       string
	NetQty    int64 // 合格接收数量 - 累计退货数量
	UnitPrice int64
}

// Ledger 是净收货账本。
type Ledger struct {
	Batches []Batch
}

// Allocation 表示一条账单行对一个收货批次的数量分配。
type Allocation struct {
	LineID  string
	BatchID string
	Qty     int64
}

// Suggestion 是一次对账建议：分配明细 + 两级目标值。
type Suggestion struct {
	Allocations  []Allocation
	MatchedQty   int64 // 总匹配数量（第一目标，最大化）
	TotalAbsDiff int64 // 总绝对价差（第二目标，在最大匹配量下最小化；单位：美分·件）
}
