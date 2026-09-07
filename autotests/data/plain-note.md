---
id: 01K6M2QW8ZP6
labels: [reference, rv-api]
---

# Auth notes

The middleware rejects a token when `exp` is in the past.

```json
{
  "exp": 1788888888,
  "sub": "user-42"
}
```

Nothing here is an action item.
