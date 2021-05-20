package model

type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
	Sku  int    `json:"sku" firestore:"sku"`
}
