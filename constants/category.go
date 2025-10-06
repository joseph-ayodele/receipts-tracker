package constants

import (
	"strings"
)

type Category string

const (
	CellPhoneService        Category = "CellPhoneService"
	HomeOffice              Category = "HomeOffice"
	Internet                Category = "Internet"
	Meals                   Category = "Meals"
	OfficeEquipment         Category = "OfficeEquipment"
	OfficeSupplies          Category = "OfficeSupplies"
	ProfessionalDevelopment Category = "ProfessionalDevelopment"
	ShippingExpenses        Category = "ShippingExpenses"
	SoftwareSubscription    Category = "SoftwareSubscription"
	TravelExpenses          Category = "TravelExpenses"
	Other                   Category = "Other"
)

var allCategories = []Category{
	CellPhoneService,
	HomeOffice,
	Internet,
	Meals,
	OfficeEquipment,
	OfficeSupplies,
	ProfessionalDevelopment,
	ShippingExpenses,
	SoftwareSubscription,
	TravelExpenses,
	Other,
}

func AsStringSlice() []string {
	result := make([]string, len(allCategories))
	for i, cat := range allCategories {
		result[i] = string(cat)
	}
	return result
}

func Canonicalize(input string) (Category, bool) {
	if input == "" {
		return Other, false
	}

	normalized := strings.ToLower(strings.TrimSpace(input))

	// synonyms map
	synonyms := map[string]Category{
		"cell phone":   CellPhoneService,
		"mobile plan":  CellPhoneService,
		"saas":         SoftwareSubscription,
		"subscription": SoftwareSubscription,
		"uber":         TravelExpenses,
		"lyft":         TravelExpenses,
		"airline":      TravelExpenses,
		"hotel":        TravelExpenses,
		"taxi":         TravelExpenses,
	}

	if cat, ok := synonyms[normalized]; ok {
		return cat, true
	}

	// check if it matches any category string
	for _, cat := range allCategories {
		if normalized == strings.ToLower(string(cat)) {
			return cat, true
		}
	}

	return Other, false
}
