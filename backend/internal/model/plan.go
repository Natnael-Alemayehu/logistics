package model

// Subscription plans. A plan caps the number of active drivers; everything
// else is the same product.
const (
	PlanTrial      = "trial"
	PlanStarter    = "starter"
	PlanGrowth     = "growth"
	PlanEnterprise = "enterprise"
)

// Plan describes a subscription tier.
type Plan struct {
	Code       string
	Name       string
	MaxDrivers int32
	// MonthlyETB is the list price; zero for the trial and for negotiated
	// enterprise contracts.
	MonthlyETB int
}

// Plans in ascending order.
var Plans = []Plan{
	{Code: PlanTrial, Name: "Free trial", MaxDrivers: 5},
	{Code: PlanStarter, Name: "Starter", MaxDrivers: 10, MonthlyETB: 2500},
	{Code: PlanGrowth, Name: "Growth", MaxDrivers: 50, MonthlyETB: 9000},
	{Code: PlanEnterprise, Name: "Enterprise", MaxDrivers: 500},
}

// PlanByCode finds a plan, reporting whether it exists.
func PlanByCode(code string) (Plan, bool) {
	for _, p := range Plans {
		if p.Code == code {
			return p, true
		}
	}
	return Plan{}, false
}

// PlanName is the display name of a plan code.
func PlanName(code string) string {
	if p, ok := PlanByCode(code); ok {
		return p.Name
	}
	return code
}
