package grocery

type GroceryList struct {
	Store API
}

func New() *GroceryList {
	return &GroceryList{&HTTPClient{}}
}

func (g *GroceryList) AddItem(item string) error {
	return g.Store.Create(&Note{Text: item})
}

func (g *GroceryList) Items() ([]string, error) {
	notes, err := g.Store.All()
	if err != nil {
		return []string{}, err
	}

	items := make([]string, len(notes))
	for i := range notes {
		items[i] = notes[i].Text
	}

	return items, nil
}
