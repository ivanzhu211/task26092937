# reconcile — 采购对账匹配库

将供应商账单行分配到实际收货批次，输出可追溯的对账建议。
只处理同一供应商、同一币种的已审核单据快照；金额为整数美分，数量为整数件，不计税费。

## 模型

- `BuildLedger(receipts, returns)`：生成净收货账本。退货严格扣减其引用的原批次，
  净数量为零的批次不保留；输出按 `BatchID` 排序，与输入顺序无关。
- `Reconcile(ledger, bills)`：返回账单行到批次的数量分配。
  不修改输入账本，可重复调用；输入顺序不影响目标值。

匹配规则：仅当账单行 SKU 与批次 SKU 相同、且批次的采购单行 ID 在账单行的
`POLineIDs` 集合内时才可匹配；集合为空则无可匹配批次。

两级目标通过最小费用最大流保证：

1. 最大化总匹配数量；
2. 在所有达到该数量的方案中，最小化总绝对价差
   （各笔分配数量 × |账单单价 − 批次单价| 之和，正负价差不抵消）。

## 运行

```sh
go build ./...   # 编译
go test ./...    # 运行全部测试
```

无外部依赖，仅使用 Go 标准库（需 Go 1.21+）。

## 示例

```go
ledger := reconcile.BuildLedger(
    []reconcile.Receipt{
        {BatchID: "B1", POLineID: "P1", SKU: "S", AcceptedQty: 10, UnitPrice: 100},
    },
    []reconcile.Return{{BatchID: "B1", Qty: 4}},
)
sg := reconcile.Reconcile(ledger, []reconcile.BillLine{
    {LineID: "A", SKU: "S", Qty: 6, UnitPrice: 104, POLineIDs: []string{"P1"}},
})
// sg.MatchedQty == 6, sg.TotalAbsDiff == 24
// sg.Allocations == [{LineID: "A", BatchID: "B1", Qty: 6}]
```
