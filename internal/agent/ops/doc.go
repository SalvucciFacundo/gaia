// Package ops provides non-SDD subagents for on-demand operations:
// Reviewer, Debugger, Researcher, and Learner.
//
// These subagents are not part of the SDD pipeline. They are dispatched
// on demand by the orchestrator when the user requests code review,
// debugging, research, or learning analysis.
//
// Each subagent follows the agent.Subagent interface and uses the
// Spawner for tool filtering, run-loop execution, and message redaction.
package ops
