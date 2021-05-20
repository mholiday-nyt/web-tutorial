package tutor3

const (
	skuDoc    = "Next$SKU"
	nextField = "next"
	startSKU  = 1000
)

type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
	SKU  int    `json:"sku" firestore:"sku"`
}
