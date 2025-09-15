package storage

import "github.com/MegaBytee/sitemap-go/types"

var tables = []interface{}{
	&types.Site{},
	&types.SitemapIndex{},
	&types.Link{},
}
