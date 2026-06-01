module github.com/tomeku/doclens/services/extraction

go 1.23

require (
	github.com/google/uuid v1.6.0
	github.com/tomeku/doclens/services/library v0.0.0-00010101000000-000000000000
	github.com/tomeku/doclens/services/shared v0.0.0-00010101000000-000000000000
)

replace github.com/tomeku/doclens/services/library => ../library

replace github.com/tomeku/doclens/services/shared => ../shared
