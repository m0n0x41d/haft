---
format: haft.terms/1
terms:
  - id: Billing.Order
    definition: A domain order with a status and integer accounting total.
    aliases: [order]
  - id: Billing.CancelableOrder
    definition: A non-nil order whose status is new or paid.
    exclusions: [shipped orders, cancelled orders]
---
The total is an integer amount in this fixture. No conversion or refund is implied.
