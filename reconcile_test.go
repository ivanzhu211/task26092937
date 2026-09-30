package reconcile

import (
	"reflect"
	"sort"
	"testing"
)

func allocQty(sg Suggestion, lineID, batchID string) int64 {
	for _, a := range sg.Allocations {
		if a.LineID == lineID && a.BatchID == batchID {
			return a.Qty
		}
	}
	return 0
}

// 争用场景：账单 A 有替代批次，账单 B 只有唯一候选。
// 若 A 贪心占用最便宜的共享批次 B1，B 将无法匹配（总匹配量 5）；
// 最优解必须重新分配：A->B2、B->B1，总匹配量 10。
func TestReconcileContentionReassignment(t *testing.T) {
	ledger := Ledger{Batches: []Batch{
		{BatchID: "B1", POLineID: "P1", SKU: "S", NetQty: 5, UnitPrice: 100},
		{BatchID: "B2", POLineID: "P2", SKU: "S", NetQty: 5, UnitPrice: 110},
	}}
	bills := []BillLine{
		{LineID: "A", SKU: "S", Qty: 5, UnitPrice: 100, POLineIDs: []string{"P1", "P2"}},
		{LineID: "B", SKU: "S", Qty: 5, UnitPrice: 100, POLineIDs: []string{"P1"}},
	}

	sg := Reconcile(ledger, bills)
	if sg.MatchedQty != 10 {
		t.Fatalf("总匹配量应为 10，得到 %d", sg.MatchedQty)
	}
	// 唯一最优：A 全部走 B2（价差 10），B 全部走 B1（价差 0）。
	if got := allocQty(sg, "A", "B2"); got != 5 {
		t.Errorf("A->B2 应为 5，得到 %d", got)
	}
	if got := allocQty(sg, "B", "B1"); got != 5 {
		t.Errorf("B->B1 应为 5，得到 %d", got)
	}
	if sg.TotalAbsDiff != 5*10 {
		t.Errorf("总绝对价差应为 50，得到 %d", sg.TotalAbsDiff)
	}
}

// 拆分场景：最大匹配量相同（5）但价差不同的两种方案：
// 3xB1 + 2xB2 => 3*10 + 2*20 = 70（最优，含拆分）
// 1xB1 + 4xB2 => 1*10 + 4*20 = 90
func TestReconcileSplitMinAbsDiff(t *testing.T) {
	ledger := Ledger{Batches: []Batch{
		{BatchID: "B1", POLineID: "P1", SKU: "S", NetQty: 3, UnitPrice: 100},
		{BatchID: "B2", POLineID: "P1", SKU: "S", NetQty: 4, UnitPrice: 130},
	}}
	bills := []BillLine{
		{LineID: "A", SKU: "S", Qty: 5, UnitPrice: 110, POLineIDs: []string{"P1"}},
	}

	sg := Reconcile(ledger, bills)
	if sg.MatchedQty != 5 {
		t.Fatalf("总匹配量应为 5，得到 %d", sg.MatchedQty)
	}
	if sg.TotalAbsDiff != 70 {
		t.Fatalf("总绝对价差应为 70，得到 %d", sg.TotalAbsDiff)
	}
	if got := allocQty(sg, "A", "B1"); got != 3 {
		t.Errorf("A->B1 应为 3，得到 %d", got)
	}
	if got := allocQty(sg, "A", "B2"); got != 2 {
		t.Errorf("A->B2 应为 2，得到 %d", got)
	}
}

// 正负价差不得相互抵消：两批一高一低，总绝对价差为绝对值之和。
func TestReconcileAbsDiffNoCancellation(t *testing.T) {
	ledger := Ledger{Batches: []Batch{
		{BatchID: "B1", POLineID: "P1", SKU: "S", NetQty: 2, UnitPrice: 90},
		{BatchID: "B2", POLineID: "P1", SKU: "S", NetQty: 2, UnitPrice: 130},
	}}
	bills := []BillLine{
		{LineID: "A", SKU: "S", Qty: 4, UnitPrice: 110, POLineIDs: []string{"P1"}},
	}
	sg := Reconcile(ledger, bills)
	if sg.MatchedQty != 4 || sg.TotalAbsDiff != 2*20+2*20 {
		t.Fatalf("期望匹配 4、价差 80，得到 %d、%d", sg.MatchedQty, sg.TotalAbsDiff)
	}
}

func TestReconcileEmptyPOLineSetNoMatch(t *testing.T) {
	ledger := Ledger{Batches: []Batch{
		{BatchID: "B1", POLineID: "P1", SKU: "S", NetQty: 5, UnitPrice: 100},
	}}
	sg := Reconcile(ledger, []BillLine{
		{LineID: "A", SKU: "S", Qty: 5, UnitPrice: 100},
	})
	if sg.MatchedQty != 0 || len(sg.Allocations) != 0 {
		t.Fatalf("空采购单行集合应无可匹配批次，得到 %+v", sg)
	}
}

// 性质：批次/账单行分配不超量；重复调用结果一致且不修改输入账本；
// 输入顺序不影响两个目标值。
func TestReconcileInvariants(t *testing.T) {
	ledger := Ledger{Batches: []Batch{
		{BatchID: "B1", POLineID: "P1", SKU: "S", NetQty: 4, UnitPrice: 100},
		{BatchID: "B2", POLineID: "P2", SKU: "S", NetQty: 6, UnitPrice: 105},
		{BatchID: "B3", POLineID: "P3", SKU: "T", NetQty: 3, UnitPrice: 200},
	}}
	bills := []BillLine{
		{LineID: "A", SKU: "S", Qty: 5, UnitPrice: 102, POLineIDs: []string{"P1", "P2"}},
		{LineID: "B", SKU: "S", Qty: 7, UnitPrice: 99, POLineIDs: []string{"P2"}},
		{LineID: "C", SKU: "T", Qty: 5, UnitPrice: 210, POLineIDs: []string{"P3"}},
	}

	before := ledger
	sg1 := Reconcile(ledger, bills)
	sg2 := Reconcile(ledger, bills)
	if !reflect.DeepEqual(sg1, sg2) {
		t.Fatalf("重复调用结果不一致:\n%+v\n%+v", sg1, sg2)
	}
	if !reflect.DeepEqual(before, ledger) {
		t.Fatalf("Reconcile 修改了输入账本")
	}

	perBatch := map[string]int64{}
	perLine := map[string]int64{}
	for _, a := range sg1.Allocations {
		perBatch[a.BatchID] += a.Qty
		perLine[a.LineID] += a.Qty
	}
	for _, bt := range ledger.Batches {
		if perBatch[bt.BatchID] > bt.NetQty {
			t.Errorf("批次 %s 分配 %d 超过净收货量 %d", bt.BatchID, perBatch[bt.BatchID], bt.NetQty)
		}
	}
	for _, bl := range bills {
		if perLine[bl.LineID] > bl.Qty {
			t.Errorf("账单行 %s 分配 %d 超过数量 %d", bl.LineID, perLine[bl.LineID], bl.Qty)
		}
	}

	// 打乱输入顺序，两个目标值必须相同。
	reversedBatches := []Batch{ledger.Batches[2], ledger.Batches[1], ledger.Batches[0]}
	reversedBills := []BillLine{bills[2], bills[1], bills[0]}
	sg3 := Reconcile(Ledger{Batches: reversedBatches}, reversedBills)
	if sg3.MatchedQty != sg1.MatchedQty || sg3.TotalAbsDiff != sg1.TotalAbsDiff {
		t.Fatalf("输入顺序影响目标值: (%d,%d) vs (%d,%d)",
			sg1.MatchedQty, sg1.TotalAbsDiff, sg3.MatchedQty, sg3.TotalAbsDiff)
	}
}

// 端到端：BuildLedger + Reconcile，验证分配可追溯且排序稳定。
func TestEndToEnd(t *testing.T) {
	ledger := BuildLedger(
		[]Receipt{
			{BatchID: "B1", POLineID: "P1", SKU: "S", AcceptedQty: 10, UnitPrice: 100},
			{BatchID: "B2", POLineID: "P1", SKU: "S", AcceptedQty: 10, UnitPrice: 108},
		},
		[]Return{{BatchID: "B1", Qty: 4}},
	)
	sg := Reconcile(ledger, []BillLine{
		{LineID: "A", SKU: "S", Qty: 12, UnitPrice: 104, POLineIDs: []string{"P1"}},
	})
	if sg.MatchedQty != 12 {
		t.Fatalf("总匹配量应为 12，得到 %d", sg.MatchedQty)
	}
	// B1 净 6（价差 4），B2 净 10（价差 4）；价差相同，任选，但总量与价差确定。
	if sg.TotalAbsDiff != 12*4 {
		t.Fatalf("总绝对价差应为 48，得到 %d", sg.TotalAbsDiff)
	}
	if !sort.SliceIsSorted(sg.Allocations, func(i, j int) bool {
		if sg.Allocations[i].LineID != sg.Allocations[j].LineID {
			return sg.Allocations[i].LineID < sg.Allocations[j].LineID
		}
		return sg.Allocations[i].BatchID < sg.Allocations[j].BatchID
	}) {
		t.Fatalf("分配输出未按 (LineID, BatchID) 排序: %+v", sg.Allocations)
	}
}
