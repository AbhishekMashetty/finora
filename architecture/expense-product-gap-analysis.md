# Expense product gap analysis

Research date: 2026-09-27. Scope deliberately excludes subscription and
recurring-bill management.

## What mature products do that Finora did not

Rocket Money and Monarch both treat transaction cleanup as a workflow rather
than a basic CRUD table. Their current product documentation describes:

- transaction rules that rename, categorize, tag, hide, or mark matching
  transactions reviewed;
- category review queues, tags, search, bulk editing, transaction splits, and
  the ability to exclude an item from budgets/cash flow without deleting it;
- budget planning that shows actual spend and remaining money in the same
  surface, plus flex/category modes and optional rollovers;
- manual CSV imports tied to a specific account, with issuer/export conventions
  handled clearly enough that users can understand what will happen before a
  bulk write.

Primary product references:

- Rocket Money, “Perfect your budget and transaction data” and “Creating
  transaction rules”: https://help.rocketmoney.com/en/collections/1964320-perfect-your-budget-and-transaction-data
- Monarch, “Creating Transaction Rules”:
  https://help.monarchmoney.com/hc/en-us/articles/360048393372-Transaction-rules
- Monarch, “Import Transaction Data Manually”:
  https://help.monarchmoney.com/hc/en-us/articles/4409682789908-Import-data-manually-from-banks-or-other-finance-apps
- Monarch, “Creating Your Budget”:
  https://help.monarchmoney.com/hc/en-us/articles/360048883631-Budgets

## Prioritized Finora roadmap

1. **Statement Import Center (implemented in this branch).** Preview the file,
   choose/detect a statement profile, support common issuer columns and sign
   conventions, then show partial-import errors clearly.
2. **Transaction review and budget exclusion.** Add `review_status` and
   `excluded_from_budget` fields, filters, bulk actions, and report behavior.
3. **Rules.** Merchant/amount/account criteria with rename, category, review,
   and exclusion actions; include a preview before applying to old data.
4. **Splits and tags.** Support multiple categorized children whose amounts
   reconcile to the original transaction, plus user-defined cross-category
   labels.
5. **Budget rollovers and flex planning.** Keep category budgets but add a
   high-level flexible-spend option and opt-in carryover balances.

## Why this order

Import quality comes first because Finora is not yet connected to an account
aggregation provider. If statement data enters incorrectly, every later budget,
report, rule, and insight is wrong. Review/exclusion comes next because it fixes
double-counted transfers and one-off expenses without destroying source data.
Rules, splits, and richer budgeting build on those two reliable foundations.
