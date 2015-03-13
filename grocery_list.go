package grocery

type GroceryList struct {
	API API
}

func New() *GroceryList {
	return &GroceryList{&HTTPClient{}}
}

func (g *GroceryList) AddItem(item string) error {
	return g.API.Create(&Note{Text: item})
}

func (g *GroceryList) Items() ([]string, error) {
	notes, err := g.API.All()
	if err != nil {
		return []string{}, err
	}

	items := make([]string, len(notes))
	for i := range notes {
		items[i] = notes[i].Text
	}

	return items, nil
}
