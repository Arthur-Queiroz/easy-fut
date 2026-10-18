package api

// FUT-005: categorias de votação. Categoria é texto no banco (sem enum);
// a validação de valores válidos é toda aqui.

var voteCategories = []string{"top_scorer", "worst_player", "best_goal"}

func validVoteCategory(c string) bool {
	for _, v := range voteCategories {
		if v == c {
			return true
		}
	}
	return false
}

// voteCategoryLabel rotula a categoria para activity logs.
func voteCategoryLabel(c string) string {
	switch c {
	case "top_scorer":
		return "artilheiro"
	case "worst_player":
		return "perna de pau"
	case "best_goal":
		return "melhor gol"
	}
	return c
}
