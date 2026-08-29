# Capability Specification: Chained PRs & Review Workload Guard

## Capability: `chained-prs`

The Chained PR planner enforces a 400-line review budget limit, automatically slicing large feature changes into reviewable PR chains (`stacked-to-main` or `feature-branch-chain`).

---

### Requirement: CHAIN-001 — Review Workload Forecast

The planner subagent MUST analyze estimated changed lines and flag tasks that exceed 400 lines.

#### Scenario: Workload within budget
- **Given** a change with estimated changed lines <= 400
- **When** the Planner creates `tasks.md`
- **Then** `Review Workload Forecast` SHALL declare `Chained PRs recommended: No` and `400-line budget risk: Low`

#### Scenario: Workload exceeds 400 lines
- **Given** a change with estimated changed lines > 400
- **When** the Planner creates `tasks.md`
- **Then** `Review Workload Forecast` SHALL declare `Chained PRs recommended: Yes` and `400-line budget risk: High`
- **And** the Planner SHALL slice tasks into distinct PR boundaries
