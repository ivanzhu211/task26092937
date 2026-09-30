package reconcile

import (
	"math/rand"
	"testing"
)

// allocQty 查询某账单行分配到某批次的数量。
func allocQty(r Reconciliation, lineID, batchID string) int64 {
	for _, a := range r.Allocations {
		if a.LineID == lineID && a.BatchID == batchID {
			return a.Qty
		}
	}
	return 0
}

// checkInvariants 校验两条必须成立的性质。
func checkInvariants(t *testing.T, ledger Ledger, bills []BillLine, r Reconciliation) {
	t.Helper()
	perBatch := map[string]int64{}
	perLine := map[string]int64{}
	for _, a := range r.Allocations {
		if a.Qty <= 0 {
			t.Fatalf("非正分配数量: %+v", a)
		}
		perBatch[a.BatchID] += a.Qty
		perLine[a.LineID] += a.Qty
	}
	for _, b := range ledger.Batches {
		if perBatch[b.BatchID] > b.NetQty {
			t.Fatalf("批次 %s 分配 %d 超过净收货量 %d", b.BatchID, perBatch[b.BatchID], b.NetQty)
		}
	}
	for _, bl := range bills {
		if perLine[bl.LineID] > bl.Qty {
			t.Fatalf("账单行 %s 分配 %d 超过其数量 %d", bl.LineID, perLine[bl.LineID], bl.Qty)
		}
	}
	var sum int64
	for _, a := range r.Allocations {
		sum += a.Qty
	}
	if sum != r.TotalMatchedQty {
		t.Fatalf("分配合计 %d 与 TotalMatchedQty %d 不一致", sum, r.TotalMatchedQty)
	}
}

// TestBuildLedger 退货必须扣减原批次，不能从同SKU其他批次扣除。
func TestBuildLedger(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "B1", POLineID: "PO1", SKU: "A", Qty: 10, UnitPriceCent: 100},
			{BatchID: "B2", POLineID: "PO1", SKU: "A", Qty: 8, UnitPriceCent: 120},
		},
		[]Return{
			{BatchID: "B1", Qty: 3},
			{BatchID: "B1", Qty: 2},
		},
	)
	if len(ledger.Batches) != 2 {
		t.Fatalf("批次数 = %d, 期望 2", len(ledger.Batches))
	}
	if ledger.Batches[0].NetQty != 5 {
		t.Fatalf("B1 净收货量 = %d, 期望 5", ledger.Batches[0].NetQty)
	}
	if ledger.Batches[1].NetQty != 8 {
		t.Fatalf("B2 净收货量 = %d, 期望 8（退货不得跨批次扣减）", ledger.Batches[1].NetQty)
	}
	if ledger.Batches[0].UnitPriceCent != 100 {
		t.Fatalf("B1 单价未保留: %d", ledger.Batches[0].UnitPriceCent)
	}
}

// TestContentionReassignment 两条账单争用批次：
// L1 有替代批次，L2 只有唯一候选。贪心让 L1 先占最便宜批次会导致 L2 无法匹配，
// 最优解必须重新分配以达到最大匹配量。
func TestContentionReassignment(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "CHEAP", POLineID: "PO1", SKU: "X", Qty: 5, UnitPriceCent: 100},
			{BatchID: "ALT", POLineID: "PO2", SKU: "X", Qty: 5, UnitPriceCent: 105},
		},
		nil,
	)
	bills := []BillLine{
		{LineID: "L1", SKU: "X", Qty: 5, UnitPriceCent: 100, POLineIDs: []string{"PO1", "PO2"}},
		{LineID: "L2", SKU: "X", Qty: 5, UnitPriceCent: 100, POLineIDs: []string{"PO1"}},
	}
	r := Reconcile(ledger, bills)
	checkInvariants(t, ledger, bills, r)

	if r.TotalMatchedQty != 10 {
		t.Fatalf("总匹配量 = %d, 期望 10（必须通过重新分配达到最大匹配量）", r.TotalMatchedQty)
	}
	// L2 只能使用 CHEAP，因此 L1 必须让出 CHEAP 改用 ALT。
	if q := allocQty(r, "L2", "CHEAP"); q != 5 {
		t.Fatalf("L2->CHEAP = %d, 期望 5", q)
	}
	if q := allocQty(r, "L1", "ALT"); q != 5 {
		t.Fatalf("L1->ALT = %d, 期望 5", q)
	}
	if r.TotalAbsPriceDiff != 25 {
		t.Fatalf("总绝对价差 = %d, 期望 25", r.TotalAbsPriceDiff)
	}
}

// TestMinAbsPriceDiffWithSplit 最大匹配量相同但总绝对价差不同的方案，
// 包含拆分分配与不同批次单价，必须选出价差最小者。
func TestMinAbsPriceDiffWithSplit(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "B1", POLineID: "PO1", SKU: "Y", Qty: 3, UnitPriceCent: 100},
			{BatchID: "B2", POLineID: "PO2", SKU: "Y", Qty: 5, UnitPriceCent: 120},
		},
		nil,
	)
	bills := []BillLine{
		{LineID: "L1", SKU: "Y", Qty: 6, UnitPriceCent: 100, POLineIDs: []string{"PO1", "PO2"}},
	}
	r := Reconcile(ledger, bills)
	checkInvariants(t, ledger, bills, r)

	if r.TotalMatchedQty != 6 {
		t.Fatalf("总匹配量 = %d, 期望 6", r.TotalMatchedQty)
	}
	// 方案A（最优）：B1分3件(价差0) + B2分3件(价差60) = 60
	// 方案B（次优）：B2分5件(价差100) + B1分1件(价差0) = 100
	if r.TotalAbsPriceDiff != 60 {
		t.Fatalf("总绝对价差 = %d, 期望 60", r.TotalAbsPriceDiff)
	}
	if q := allocQty(r, "L1", "B1"); q != 3 {
		t.Fatalf("L1->B1 = %d, 期望 3", q)
	}
	if q := allocQty(r, "L1", "B2"); q != 3 {
		t.Fatalf("L1->B2 = %d, 期望 3（拆分分配）", q)
	}
}

// TestEmptyPOLineIDs 允许集合为空时无可匹配批次。
func TestEmptyPOLineIDs(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{{BatchID: "B1", POLineID: "PO1", SKU: "A", Qty: 10, UnitPriceCent: 100}},
		nil,
	)
	r := Reconcile(ledger, []BillLine{
		{LineID: "L1", SKU: "A", Qty: 4, UnitPriceCent: 100},
	})
	if r.TotalMatchedQty != 0 || len(r.Allocations) != 0 {
		t.Fatalf("空允许集合不应匹配: %+v", r)
	}
}

// TestSKUAndPOFiltering 只有SKU相同且采购单行ID在允许集合中的批次可参与匹配。
func TestSKUAndPOFiltering(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "B1", POLineID: "PO1", SKU: "A", Qty: 10, UnitPriceCent: 100},
			{BatchID: "B2", POLineID: "PO2", SKU: "A", Qty: 10, UnitPriceCent: 100},
			{BatchID: "B3", POLineID: "PO1", SKU: "B", Qty: 10, UnitPriceCent: 100},
		},
		nil,
	)
	bills := []BillLine{
		{LineID: "L1", SKU: "A", Qty: 15, UnitPriceCent: 100, POLineIDs: []string{"PO1"}},
	}
	r := Reconcile(ledger, bills)
	checkInvariants(t, ledger, bills, r)
	if r.TotalMatchedQty != 10 {
		t.Fatalf("总匹配量 = %d, 期望 10（仅 B1 可匹配）", r.TotalMatchedQty)
	}
	if q := allocQty(r, "L1", "B1"); q != 10 {
		t.Fatalf("L1->B1 = %d, 期望 10", q)
	}
}

// TestLedgerNotConsumed 重复调用不得消耗或修改输入账本。
func TestLedgerNotConsumed(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{{BatchID: "B1", POLineID: "PO1", SKU: "A", Qty: 10, UnitPriceCent: 100}},
		nil,
	)
	bills := []BillLine{
		{LineID: "L1", SKU: "A", Qty: 10, UnitPriceCent: 100, POLineIDs: []string{"PO1"}},
	}
	r1 := Reconcile(ledger, bills)
	r2 := Reconcile(ledger, bills)
	if r1.TotalMatchedQty != 10 || r2.TotalMatchedQty != 10 {
		t.Fatalf("重复调用结果异常: %d, %d", r1.TotalMatchedQty, r2.TotalMatchedQty)
	}
	if ledger.Batches[0].NetQty != 10 {
		t.Fatalf("输入账本被修改: NetQty = %d", ledger.Batches[0].NetQty)
	}
}

// TestInputOrderIndependence 输入顺序不得影响两个目标值。
func TestInputOrderIndependence(t *testing.T) {
	receipts := []Receipt{
		{BatchID: "B1", POLineID: "PO1", SKU: "A", Qty: 4, UnitPriceCent: 100},
		{BatchID: "B2", POLineID: "PO2", SKU: "A", Qty: 6, UnitPriceCent: 110},
		{BatchID: "B3", POLineID: "PO3", SKU: "A", Qty: 5, UnitPriceCent: 95},
		{BatchID: "B4", POLineID: "PO4", SKU: "B", Qty: 7, UnitPriceCent: 200},
	}
	bills := []BillLine{
		{LineID: "L1", SKU: "A", Qty: 8, UnitPriceCent: 105, POLineIDs: []string{"PO1", "PO2", "PO3"}},
		{LineID: "L2", SKU: "A", Qty: 5, UnitPriceCent: 100, POLineIDs: []string{"PO2", "PO3"}},
		{LineID: "L3", SKU: "B", Qty: 9, UnitPriceCent: 190, POLineIDs: []string{"PO4"}},
	}
	base := Reconcile(BuildLedger(receipts, nil), bills)

	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 20; trial++ {
		rs := append([]Receipt(nil), receipts...)
		bs := append([]BillLine(nil), bills...)
		rng.Shuffle(len(rs), func(i, j int) { rs[i], rs[j] = rs[j], rs[i] })
		rng.Shuffle(len(bs), func(i, j int) { bs[i], bs[j] = bs[j], bs[i] })
		got := Reconcile(BuildLedger(rs, nil), bs)
		if got.TotalMatchedQty != base.TotalMatchedQty ||
			got.TotalAbsPriceDiff != base.TotalAbsPriceDiff {
			t.Fatalf("第 %d 次乱序: 目标值 (%d,%d), 期望 (%d,%d)",
				trial, got.TotalMatchedQty, got.TotalAbsPriceDiff,
				base.TotalMatchedQty, base.TotalAbsPriceDiff)
		}
	}
}

// TestAbsDiffNoCancellation 正负价差不能相互抵消。
func TestAbsDiffNoCancellation(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "LOW", POLineID: "PO1", SKU: "A", Qty: 2, UnitPriceCent: 90},
			{BatchID: "HIGH", POLineID: "PO2", SKU: "A", Qty: 2, UnitPriceCent: 110},
		},
		nil,
	)
	bills := []BillLine{
		{LineID: "L1", SKU: "A", Qty: 2, UnitPriceCent: 100, POLineIDs: []string{"PO1"}},
		{LineID: "L2", SKU: "A", Qty: 2, UnitPriceCent: 100, POLineIDs: []string{"PO2"}},
	}
	r := Reconcile(ledger, bills)
	checkInvariants(t, ledger, bills, r)
	// 价差 -10 与 +10 各 2 件，绝对值之和为 40，不得抵消为 0。
	if r.TotalAbsPriceDiff != 40 {
		t.Fatalf("总绝对价差 = %d, 期望 40", r.TotalAbsPriceDiff)
	}
}
