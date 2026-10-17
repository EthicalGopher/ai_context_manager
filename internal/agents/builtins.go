package agents

// ContextPrompt is the short initial prompt used when a CLI accepts one. The
// full package lives in a file so argv stays small and nothing is re-parsed.
const ContextPrompt = "Read the project context package at {contextFile} before doing anything else. Treat it as a starting point and verify claims against the repository. Then wait for my instructions."

// Builtins returns the built-in adapters. "Verified" means the flags were
// checked against the installed CLI's own --help on the stated version.
// Unverified adapters document their source and are labelled as such in the UI.
func Builtins() []Profile {
	return []Profile{
		{
			ID: "claude", DisplayName: "Claude Code", Executables: []string{"claude"}, Builtin: true,
			ModelArgs: []string{"--model", "{model}"}, ModelHints: []string{"sonnet", "opus", "haiku"},
			Docs:          "https://docs.claude.com/en/docs/claude-code/cli-reference",
			Caps:          Capabilities{NewSession: true, ResumeByID: true, ContinueLast: true, AssignSessionID: true, InitialPrompt: true, ReadsInstrFiles: true, NonInteractive: true, PermissionModes: true},
			SessionIDArgs: []string{"--session-id", "{session}"}, ResumeArgs: []string{"--resume", "{session}"}, ContinueArgs: []string{"--continue"},
			PromptArgs: []string{ContextPrompt}, Handoff: HandoffPromptArg, Verified: true, VerifiedVersion: "2.1.296",
			Notes: "Reads CLAUDE.md. Initial prompt is a positional argument; session IDs can be pre-assigned with --session-id.",
			Chat:  &ChatSpec{NewArgs: []string{"-p"}, ResumeArgs: []string{"-p", "--resume", "{session}"}, SessionIDArgs: []string{"--session-id", "{session}"}, EditArgs: []string{"--permission-mode", "acceptEdits"}, PromptVia: "stdin", AssignSessionID: true},
		},
		{
			ID: "codex", DisplayName: "OpenAI Codex CLI", Executables: []string{"codex"}, Builtin: true,
			ModelArgs:  []string{"--model", "{model}"},
			Docs:       "https://github.com/openai/codex",
			Caps:       Capabilities{NewSession: true, ResumeByID: true, ContinueLast: true, InitialPrompt: true, ReadsInstrFiles: true, NonInteractive: true, PermissionModes: true, WorkingDirFlag: true},
			ResumeArgs: []string{"resume", "{session}"}, ContinueArgs: []string{"resume", "--last"},
			PromptArgs: []string{ContextPrompt}, Handoff: HandoffPromptArg, Verified: false,
			Notes: "Flags taken from upstream documentation; not installed on the development machine, so NOT verified. Codex does not accept a caller-chosen session ID.",
		},
		{
			ID: "gemini", DisplayName: "Gemini CLI", Executables: []string{"gemini"}, Builtin: true,
			ModelArgs:  []string{"--model", "{model}"},
			Docs:       "https://github.com/google-gemini/gemini-cli",
			Caps:       Capabilities{NewSession: true, InitialPrompt: true, ReadsInstrFiles: true, NonInteractive: true, PermissionModes: true},
			PromptArgs: []string{"-i", ContextPrompt}, Handoff: HandoffPromptArg, Verified: false,
			Notes: "Flags taken from upstream documentation; not installed on the development machine, so NOT verified. No resume mechanism is claimed.",
		},
		{
			ID: "kilo", DisplayName: "Kilo", Executables: []string{"kilo", "kilocode"}, Builtin: true,
			ModelArgs:  []string{"--model", "{model}"},
			Docs:       "https://kilo.ai/docs",
			Caps:       Capabilities{NewSession: true, ResumeByID: true, ContinueLast: true, InitialPrompt: true, ReadsInstrFiles: true, NonInteractive: true, PermissionModes: true},
			ResumeArgs: []string{"--session", "{session}"}, ContinueArgs: []string{"--continue"},
			PromptArgs: []string{"--prompt", ContextPrompt}, Handoff: HandoffPromptArg, Verified: true, VerifiedVersion: "7.4.21",
			Notes: "Verified from `kilo --help`: --prompt, --session, --continue. Kilo chooses its own session IDs. Models are provider/model; run `kilo models` to list them, including local providers you configured.",
			Chat:  &ChatSpec{NewArgs: []string{"run", "{prompt}"}, ContinueArgs: []string{"run", "--continue", "{prompt}"}, EditArgs: []string{"--auto"}},
		},
		{
			ID: "agy", DisplayName: "AGY", Executables: []string{"agy"}, Builtin: true,
			ModelArgs:  []string{"--model", "{model}"},
			Docs:       "run `agy --help`",
			Caps:       Capabilities{NewSession: true, ResumeByID: true, ContinueLast: true, InitialPrompt: true, NonInteractive: true, PermissionModes: true},
			ResumeArgs: []string{"--conversation", "{session}"}, ContinueArgs: []string{"--continue"},
			PromptArgs: []string{"--prompt-interactive", ContextPrompt}, Handoff: HandoffPromptArg, Verified: true, VerifiedVersion: "1.2.14",
			Notes: "Verified from `agy --help`: --prompt-interactive, --conversation, --continue. AGY chooses its own conversation IDs.",
			Chat:  &ChatSpec{NewArgs: []string{"--print", "{prompt}"}, ContinueArgs: []string{"--continue", "--print", "{prompt}"}, EditArgs: []string{"--mode", "accept-edits"}},
		},
		{
			ID: "freebuff", DisplayName: "Freebuff", Executables: []string{"freebuff"}, Builtin: true,
			Docs:       "run `freebuff --help`",
			Caps:       Capabilities{NewSession: true, ResumeByID: true, ContinueLast: true, WorkingDirFlag: true},
			ResumeArgs: []string{"--continue", "{session}"}, ContinueArgs: []string{"--continue"},
			Handoff: HandoffFile, Verified: true, VerifiedVersion: "0.0.163",
			Notes: "Verified from `freebuff --help`: --continue [conversation-id], --cwd. It has no initial-prompt option, so context is handed over as a file.",
		},
	}
}
