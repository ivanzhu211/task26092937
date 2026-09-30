package reconcile

// BuildLedger 由收货与退货记录生成净收货账本。
// 退货按批次ID扣减原批次，不跨批次扣减；净收货量为负的批次按0处理。
func BuildLedger(receipts []Receipt, returns []Return) Ledger {
	returned := make(map[string]int64, len(returns))
	for _, r := range returns {
		returned[r.BatchID] += r.Qty
	}

	batches := make([]Batch, 0, len(receipts))
	for _, rc := range receipts {
		net := rc.Qty - returned[rc.BatchID]
		if net < 0 {
			net = 0
		}
		batches = append(batches, Batch{
			BatchID:       rc.BatchID,
			POLineID:      rc.POLineID,
			SKU:           rc.SKU,
			NetQty:        net,
			UnitPriceCent: rc.UnitPriceCent,
		})
	}
	return Ledger{Batches: batches}
}
