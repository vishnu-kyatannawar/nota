---
id: 01K6M2QW8ZP4
type: workplan
date: 2026-09-02
hours: "01:20"
daytype: work
labels: [work]
---

- [ ] Check calendar #daily <!--n id:01K6M2R0 t:08:55 rec:daily-->
- [x] Fix auth token expiry #rv-api [01:20] <!--n id:01K6M2R4 t:09:34 done:11:02-->
      Middleware compares `exp < now`, which is off by one on the boundary second.

      ```go
      if exp <= now {
          return ErrExpired
      }
      ```

- [ ] Review PR 412 #rv-portal <!--n id:01K6J8XX t:09:40 from:2026-09-01 carried:1-->
  - [ ] Check the migration <!--n id:01K6J8XY t:09:41-->
  - [x] Read the description <!--n id:01K6J8XZ t:09:42 done:09:55-->

## Notes

Free-form prose lives here, below the action items.
