package reconcile

import "sort"

// BuildLedger 由收货与退货记录生成净收货账本。
// 退货必须扣减其引用的原批次，不能从同SKU的其他批次扣除。
// 净数量为零的批次不保留。输出按 BatchID 排序，与输入顺序无关。
func BuildLedger(receipts []Receipt, returns []Return) Ledger {
	returned := make(map[string]int64, len(returns))
	for _, r := range returns {
		returned[r.BatchID] += r.Qty
	}

	batches := make([]Batch, 0, len(receipts))
	for _, rc := range receipts {
		net := rc.AcceptedQty - returned[rc.BatchID]
		if net <= 0 {
			continue
		}
		batches = append(batches, Batch{
			BatchID:   rc.BatchID,
			POLineID:  rc.POLineID,
			SKU:       rc.SKU,
			NetQty:    net,
			UnitPrice: rc.UnitPrice,
		})
	}

	sort.Slice(batches, func(i, j int) bool { return batches[i].BatchID < batches[j].BatchID })
	return Ledger{Batches: batches}
}
