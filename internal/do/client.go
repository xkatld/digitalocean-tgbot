package do

import (
	"context"
	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

type Client struct {
	*godo.Client
}

func NewClient(token string) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	return &Client{godo.NewClient(oauthClient)}
}

func (c *Client) GetAccount(ctx context.Context) (*godo.Account, error) {
	account, _, err := c.Account.Get(ctx)
	return account, err
}

func (c *Client) ListRegions(ctx context.Context) ([]godo.Region, error) {
	opt := &godo.ListOptions{PerPage: 200}
	regions, _, err := c.Regions.List(ctx, opt)
	return regions, err
}

func (c *Client) ListSizes(ctx context.Context) ([]godo.Size, error) {
	opt := &godo.ListOptions{PerPage: 200}
	sizes, _, err := c.Sizes.List(ctx, opt)
	return sizes, err
}

func (c *Client) ListDistributions(ctx context.Context) ([]godo.Image, error) {
	opt := &godo.ListOptions{PerPage: 200}
	images, _, err := c.Images.ListDistribution(ctx, opt)
	return images, err
}

func (c *Client) CreateDroplet(ctx context.Context, name, region, size, image string, userData string) (*godo.Droplet, error) {
	createRequest := &godo.DropletCreateRequest{
		Name:     name,
		Region:   region,
		Size:     size,
		Image:    godo.DropletCreateImage{Slug: image},
		UserData: userData,
	}
	droplet, _, err := c.Droplets.Create(ctx, createRequest)
	return droplet, err
}

func (c *Client) GetDroplet(ctx context.Context, id int) (*godo.Droplet, error) {
	droplet, _, err := c.Droplets.Get(ctx, id)
	return droplet, err
}

func (c *Client) ListDroplets(ctx context.Context) ([]godo.Droplet, error) {
	opt := &godo.ListOptions{PerPage: 200}
	list, _, err := c.Droplets.List(ctx, opt)
	return list, err
}

func (c *Client) DeleteDroplet(ctx context.Context, id int) error {
	_, err := c.Droplets.Delete(ctx, id)
	return err
}
