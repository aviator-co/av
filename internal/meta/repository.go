package meta

type Repository struct {
	// The GitHub (GraphQL) ID of the repository (e.g., R_kgDOHMmHmg).
	ID string `json:"id"`
	// The owner of the repository (e.g., aviator-co)
	Owner string `json:"owner"`
	// The name of the repository (e.g., av)
	Name string `json:"name"`
}
