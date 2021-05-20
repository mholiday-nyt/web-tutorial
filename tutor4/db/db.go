package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"tutor4/graph/model"
)

type DB interface {
	AddItem(context.Context, *model.Item) (string, error)
	GetItem(context.Context, string) (*model.Item, error)
	GetItemBySKU(context.Context, int) (*model.Item, error)
	ListItems(context.Context) ([]*model.Item, error)
	ListSKUs(context.Context) (map[string]string, error)
	UpdateItem(context.Context, *model.Item) error
	DeleteItem(context.Context, string) error
}

const (
	skuDoc    = "Next$SKU"
	nextField = "next"
	startSKU  = 1000
)

type Client struct {
	fs   *firestore.Client
	data *firestore.CollectionRef
	util *firestore.CollectionRef
}

func NewClient(project, data, util string) (*Client, error) {
	if project == "" {
		return nil, errors.New("no projectID")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, project)

	if err != nil {
		return nil, fmt.Errorf("failed to create FS client: %w", err)
	}

	c := Client{
		fs:   client,
		data: client.Collection(data),
		util: client.Collection(util),
	}

	if err = c.startSKU(ctx); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Client) Close() {
	c.fs.Close()
}

func (c *Client) startSKU(ctx context.Context) error {
	ref := c.util.Doc(skuDoc)

	return c.fs.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)

		if err != nil {
			if status.Code(err) == codes.NotFound {
				log.Println("no SKU doc, adding it")

				data := map[string]interface{}{
					nextField: startSKU,
				}

				if err := tx.Create(ref, data); err != nil {
					return fmt.Errorf("add SKU failed: %s", err)
				}

				return nil
			}

			return err
		}

		valRef, err := doc.DataAt(nextField)

		if err != nil {
			return err
		}

		if val, ok := valRef.(int64); ok {
			log.Printf("started SKU, %s = %v", nextField, val)
			return nil
		}

		return fmt.Errorf("can't read %s: %v", nextField, doc.Data())
	})
}

func getNext(seqRef *firestore.DocumentRef, tx *firestore.Transaction) (int, error) {
	seq, err := tx.Get(seqRef) // tx.Get, NOT docRef.Get!

	if err != nil {
		return 0, err
	}

	valRef, err := seq.DataAt(nextField)

	if err != nil {
		return 0, err
	}

	val, ok := valRef.(int64)

	if !ok {
		return 0, fmt.Errorf("can't read %s", nextField)
	}

	return int(val), nil
}

func (c *Client) create(ctx context.Context, ref *firestore.DocumentRef, item *model.Item) error {
	seqRef := c.util.Doc(skuDoc)

	return c.fs.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		next, err := getNext(seqRef, tx)

		if err != nil {
			return err
		}

		item.Sku = next

		// if the transaction fails, this write will
		// also fail, so we shouldn't waste SKUs

		if err := tx.Update(seqRef, []firestore.Update{{Path: nextField, Value: next + 1}}); err != nil {
			return err
		}

		// using Create here will prevent overwriting an
		// existing offer with the same UUID

		if err := tx.Create(ref, item); err != nil {
			return err
		}

		return nil
	})
}

var ErrNotFound = errors.New("not found")

func (c *Client) AddItem(ctx context.Context, i *model.Item) (string, error) {
	var ref *firestore.DocumentRef

add:
	i.ID = uuid.New().String()
	ref = c.data.Doc(i.ID)

	if err := c.create(ctx, ref, i); err != nil {
		// it's unlikely to happen even once and virtually
		// impossible for it to happen twice in a row

		if status.Code(err) == codes.AlreadyExists {
			goto add
		}

		return "", err
	}

	return i.ID, nil
}

func (c *Client) GetItem(ctx context.Context, id string) (*model.Item, error) {
	doc, err := c.data.Doc(id).Get(ctx)

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("%s: %w", id, ErrNotFound)
		}

		return nil, fmt.Errorf("item %s: %w", id, err)
	}

	var i model.Item

	if err = doc.DataTo(&i); err != nil {
		return nil, fmt.Errorf("item %s decode: %w", id, err)
	}

	return &i, nil
}

func (c *Client) GetItemBySKU(ctx context.Context, sku int) (*model.Item, error) {
	query := c.data.Where("sku", "==", sku)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		log.Printf("error finding sku %d: %s", sku, err)
		return nil, err
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("sku %d: %w", sku, ErrNotFound)
	}

	var i model.Item

	if err = docs[0].DataTo(&i); err != nil {
		log.Printf("item %s decode: %s", docs[0].Ref.ID, err)
		return nil, err
	}

	return &i, nil
}

func (c *Client) ListItems(ctx context.Context) ([]*model.Item, error) {
	query := c.data.OrderBy(firestore.DocumentID, firestore.Asc)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		return nil, err
	}

	result := make([]*model.Item, 0, len(docs))

	for _, doc := range docs {
		var i model.Item

		if err = doc.DataTo(&i); err != nil {
			log.Printf("item %s decode: %s", doc.Ref.ID, err)
			continue
		}

		result = append(result, &i)
	}

	return result, nil
}

func (c *Client) ListSKUs(ctx context.Context) (map[string]string, error) {
	query := c.data.OrderBy("sku", firestore.Asc)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(docs))

	for _, doc := range docs {
		var i model.Item

		if err = doc.DataTo(&i); err != nil {
			log.Printf("item %s decode: %s", doc.Ref.ID, err)
			continue
		}

		sku := strconv.Itoa(i.Sku)

		result[sku] = i.ID
	}

	return result, nil
}

func (c *Client) UpdateItem(ctx context.Context, i *model.Item) error {
	ref := c.data.Doc(i.ID)

	// set can create or overwrite existing data
	// so we need to see if it exists first

	if _, err := ref.Get(ctx); err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("%s: %w", i.ID, ErrNotFound)
		}

		return err
	}

	if _, err := ref.Set(ctx, i); err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteItem(ctx context.Context, id string) error {
	_, err := c.data.Doc(id).Delete(ctx)

	if err != nil {
		return err
	}

	return nil
}
