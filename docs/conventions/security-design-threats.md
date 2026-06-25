# 安全設計:初版必列雙向威脅情境

提 rate-limit / 帳號鎖定 / 存取控制等安全機制設計時,**初版 BDD 必須同時包含「攻擊者濫用此控制」與「合法使用者被連坐」兩面的情境**,不要等使用者推回來才補。

## Why

2026-05-21 做 Calitor 登入鎖定改造,初版設計連續被推回兩次:

1. 「by 帳號跨 IP 鎖」 → 使用者立刻指出「攻擊者可從遠端對任意公司帳號狂試錯密碼,鎖死正常使用者」(by-account DoS 攻擊面)
2. 改成「`(帳號, IP)` 組合」 → 使用者又問了一個我沒列的 edge case:同 IP 有 1 次「存在帳號錯密碼」+ 4 次「不存在帳號」,第 6 次會怎樣?

兩次都是可預期的 threat model 缺口,應該在初版 plan 的 BDD 區就列為 Scenario,而不是等使用者找出來。

**最終修復**:commit `9f5234d` (2026-05-21)。Middleware 取請求 body 的帳號,維護兩個獨立 Redis counter:
- `login_limit:user:<account>:<ip>` 計密碼錯誤(成功登入時清零)
- `login_limit:nonexistent:<ip>` 計不存在帳號嘗試(**成功登入不清零**,避免攻擊者用一次成功登入清掉枚舉計數)

## How to apply

任何 rate-limit / lock / 存取控制提案,BDD 區必須涵蓋以下四個面向的 Scenario:

### (a) 攻擊者用此控制 DoS 合法使用者
例如:把別人帳號鎖死、把整個辦公室 IP 鎖死。
- 反例:純 by-account 鎖 → 攻擊者鎖死任意公司帳號
- 反例:純 by-IP 鎖 → 同辦公室 NAT 後所有員工互鎖

### (b) 攻擊者繞過此控制的維度
- 列舉攻擊(嘗試大量「不同」帳號看哪些存在)
- 廣度撞庫(分散到多個 IP)
- 變化大小寫繞過正規化(帳號是否 case-sensitive?)

### (c) 共用 key 維度的連坐傷害
- 同 IP 共用(辦公室、學校、行動網路 NAT)
- 同帳號被多人使用(共用收銀帳號、客服輪班)

### (d) 多重控制的交互效應
**計數 A 半滿 + 計數 B 半滿 = 是否觸發某種隱含狀態?**
- 例:`(帳號,IP)` 計數 + 「不存在帳號」計數 兩個獨立 counter,要分開講清楚各自的清零條件,以及攻擊者能否用一個 counter 的成功事件清掉另一個
- commit `9f5234d` 的關鍵設計就是「成功登入只清 (帳號,IP) counter,**不清** nonexistent counter」 — 這是 (d) 的具體應用

## Checklist 樣板(寫安全設計 plan 時可直接抄)

```markdown
## 威脅情境(BDD Scenario)

### (a) DoS 合法使用者
Given … When … Then …

### (b) 攻擊者繞過維度
Given … When … Then …

### (c) 共用 key 連坐
Given … When … Then …

### (d) 多重控制交互
Given … When … Then …
```

每個 Scenario 都應有對應的緩解措施寫在 plan 的「技術選擇」或「修改範圍」區。
