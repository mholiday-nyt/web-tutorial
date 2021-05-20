package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"tutor4/graph/generated"
	"tutor4/graph/model"
)

func (r *mutationResolver) CreateItem(ctx context.Context, input model.NewItem) (*model.Item, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("no name")
	}

	item := model.Item{
		Name: input.Name,
	}

	_, err := r.Client.AddItem(ctx, &item)

	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (r *queryResolver) Items(ctx context.Context) ([]*model.Item, error) {
	items, err := r.Client.ListItems(ctx)

	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *queryResolver) Item(ctx context.Context, sku int) (*model.Item, error) {
	item, err := r.Client.GetItemBySKU(ctx, sku)

	if err != nil {
		return nil, err
	}

	return item, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
