package cli

import (
	"os"
	"strings"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"

	"github.com/spf13/cobra"
)

func newEmotionCommand() *cobra.Command {
	var workspacePath string
	command := &cobra.Command{
		Use:   "emotion",
		Short: "emotion 情绪状态命令",
	}
	command.PersistentFlags().StringVar(&workspacePath, "workspace", "", "workspace absolute path")

	resolveWorkspace := func() (string, error) {
		workspace := strings.TrimSpace(workspacePath)
		if workspace == "" {
			workspace = strings.TrimSpace(os.Getenv(nexusctlWorkspacePathEnvName))
		}
		if workspace == "" {
			return "", usageErrorf("emotion requires --workspace or %s", nexusctlWorkspacePathEnvName)
		}
		return workspace, nil
	}

	command.AddCommand(func() *cobra.Command {
		var contextID string
		statusCommand := &cobra.Command{
			Use:   "status",
			Short: "读取当前情绪状态",
			RunE: func(cmd *cobra.Command, args []string) error {
				workspace, err := resolveWorkspace()
				if err != nil {
					return err
				}
				view := agentsvc.LoadRuntimeEmotionView(workspace, contextID, time.Now())
				return emitJSON(map[string]any{
					"domain": "emotion",
					"action": "status",
					"view":   view,
				})
			},
		}
		statusCommand.Flags().StringVar(&contextID, "context-id", "", "emotion context id")
		return statusCommand
	}())

	command.AddCommand(newEmotionResetCommand(resolveWorkspace))
	command.AddCommand(newEmotionNoteCommand(resolveWorkspace))
	command.AddCommand(newEmotionClearCommand(resolveWorkspace))

	return command
}

func newEmotionResetCommand(resolveWorkspace func() (string, error)) *cobra.Command {
	var mood string
	var energy int
	var valence int
	var note string
	resetCommand := &cobra.Command{
		Use:   "reset",
		Short: "重置长期情绪状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := resolveWorkspace()
			if err != nil {
				return err
			}
			if err = requireNonEmptyFlag("mood", mood); err != nil {
				return err
			}
			if err = validateEmotionScore(energy, "energy"); err != nil {
				return err
			}
			if err = validateEmotionScore(valence, "valence"); err != nil {
				return err
			}
			if err = requireNonEmptyFlag("note", note); err != nil {
				return err
			}
			view, err := agentsvc.SetRuntimeEmotionBase(workspace, agentsvc.RuntimeEmotionBaseUpdate{
				Mood:        mood,
				Energy:      energy,
				Valence:     valence,
				Description: note,
				Timestamp:   time.Now(),
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "emotion",
				"action": "reset_base",
				"view":   view,
			})
		},
	}
	resetCommand.Flags().StringVar(&mood, "mood", "", "base mood label")
	resetCommand.Flags().IntVar(&energy, "energy", -1, "base energy score, 0-10")
	resetCommand.Flags().IntVar(&valence, "valence", -1, "base valence score, 0-10")
	resetCommand.Flags().StringVar(&note, "note", "", "base mood note")
	return resetCommand
}

func newEmotionNoteCommand(resolveWorkspace func() (string, error)) *cobra.Command {
	var contextID string
	var mood string
	var valence int
	var reason string
	noteCommand := &cobra.Command{
		Use:   "note",
		Short: "记录当前上下文情绪",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := resolveWorkspace()
			if err != nil {
				return err
			}
			if err = requireNonEmptyFlag("context-id", contextID); err != nil {
				return err
			}
			if err = requireNonEmptyFlag("mood", mood); err != nil {
				return err
			}
			if err = validateEmotionScore(valence, "valence"); err != nil {
				return err
			}
			if err = requireNonEmptyFlag("reason", reason); err != nil {
				return err
			}
			view, err := agentsvc.SetRuntimeEmotionContext(workspace, agentsvc.RuntimeEmotionContextUpdate{
				ContextID: contextID,
				Mood:      mood,
				Valence:   valence,
				Trigger:   reason,
				Timestamp: time.Now(),
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "emotion",
				"action": "record_context",
				"view":   view,
			})
		},
	}
	noteCommand.Flags().StringVar(&contextID, "context-id", "", "emotion context id")
	noteCommand.Flags().StringVar(&mood, "mood", "", "context mood label")
	noteCommand.Flags().IntVar(&valence, "valence", -1, "context valence score, 0-10")
	noteCommand.Flags().StringVar(&reason, "reason", "", "why this context mood changed")
	return noteCommand
}

func newEmotionClearCommand(resolveWorkspace func() (string, error)) *cobra.Command {
	var contextID string
	clearCommand := &cobra.Command{
		Use:   "clear",
		Short: "清除上下文情绪",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := resolveWorkspace()
			if err != nil {
				return err
			}
			if err = requireNonEmptyFlag("context-id", contextID); err != nil {
				return err
			}
			view, err := agentsvc.ClearRuntimeEmotionContext(workspace, contextID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "emotion",
				"action": "clear_context",
				"view":   view,
			})
		},
	}
	clearCommand.Flags().StringVar(&contextID, "context-id", "", "emotion context id")
	return clearCommand
}

func validateEmotionScore(score int, field string) error {
	if score < 0 || score > 10 {
		return usageErrorf("%s must be from 0 to 10", field)
	}
	return nil
}

func requireNonEmptyFlag(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return usageErrorf("emotion requires --%s", name)
	}
	return nil
}
