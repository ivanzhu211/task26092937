package reconcile

import "testing"

func TestBuildLedgerNetQuantities(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "B1", POLineID: "P1", SKU: "S", AcceptedQty: 10, UnitPrice: 100},
			{BatchID: "B2", POLineID: "P1", SKU: "S", AcceptedQty: 4, UnitPrice: 120},
			{BatchID: "B3", POLineID: "P2", SKU: "S", AcceptedQty: 3, UnitPrice: 90},
		},
		[]Return{
			{BatchID: "B1", Qty: 2},
			{BatchID: "B1", Qty: 3},
			{BatchID: "B3", Qty: 3}, // 全部退掉，净数量为零应被剔除
		},
	)

	if len(ledger.Batches) != 2 {
		t.Fatalf("期望 2 个净批次，得到 %d", len(ledger.Batches))
	}
	// 输出按 BatchID 排序。
	if ledger.Batches[0].BatchID != "B1" || ledger.Batches[0].NetQty != 5 || ledger.Batches[0].UnitPrice != 100 {
		t.Errorf("B1 净批次不符: %+v", ledger.Batches[0])
	}
	// 退货只扣减原批次 B1，不得影响同 SKU 的 B2。
	if ledger.Batches[1].BatchID != "B2" || ledger.Batches[1].NetQty != 4 {
		t.Errorf("B2 净批次不符: %+v", ledger.Batches[1])
	}
}

func TestBuildLedgerOrderIndependent(t *testing.T) {
	receipts := []Receipt{
		{BatchID: "B2", POLineID: "P1", SKU: "S", AcceptedQty: 4, UnitPrice: 120},
		{BatchID: "B1", POLineID: "P1", SKU: "S", AcceptedQty: 10, UnitPrice: 100},
	}
	a := BuildLedger(receipts, nil)
	b := BuildLedger([]Receipt{receipts[1], receipts[0]}, nil)
	if len(a.Batches) != len(b.Batches) {
		t.Fatalf("输入顺序改变了账本内容")
	}
	for i := range a.Batches {
		if a.Batches[i] != b.Batches[i] {
			t.Fatalf("输入顺序改变了账本内容: %v vs %v", a.Batches, b.Batches)
		}
	}
}
