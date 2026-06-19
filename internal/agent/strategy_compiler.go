package agent

import (
	"strings"
	"trade/internal/models"
)

type StrategyCompiler struct{}

func (c *StrategyCompiler) Compile(intent *models.UserIntent) *models.StrategyRules {
	rules := &models.StrategyRules{
		AssetFilter:  c.buildAssetFilter(intent.Assets),
		PositionSize: c.buildPositionSize(intent.RiskProfile),
	}

	entryConditions := []models.Condition{}
	if intent.MarketCondition == "bullish" {
		entryConditions = append(entryConditions, models.Condition{
			Type: "technical", Operator: "==", Threshold: 0,
		})
	}
	for _, signal := range intent.EntrySignals {
		entryConditions = append(entryConditions, models.Condition{
			Type: signal, Operator: ">", Threshold: 0.6,
		})
	}
	rules.EntryCondition = models.Condition{
		Type: "and", SubConditions: entryConditions,
	}

	exitConditions := []models.Condition{}
	for _, signal := range intent.ExitSignals {
		exitConditions = append(exitConditions, models.Condition{
			Type: signal, Operator: "<", Threshold: 0.3,
		})
	}
	rules.ExitCondition = models.Condition{
		Type: "or", SubConditions: exitConditions,
	}

	return rules
}

func (c *StrategyCompiler) buildAssetFilter(assets []string) string {
	if len(assets) == 0 {
		return "all"
	}
	return strings.Join(assets, ",")
}

func (c *StrategyCompiler) buildPositionSize(profile string) string {
	switch profile {
	case "low":
		return "0.01"
	case "medium":
		return "0.05"
	case "aggressive":
		return "0.1"
	default:
		return "0.01"
	}
}
