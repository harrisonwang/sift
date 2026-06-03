// Package all blank-imports every concrete provider so their init()
// self-registration runs. Import this package (for side effects) from any entry
// point that needs the full provider set available in the registry.
package all

import (
	_ "github.com/harrisonwang/sift/internal/provider/hackernews"
	_ "github.com/harrisonwang/sift/internal/provider/reddit"
	_ "github.com/harrisonwang/sift/internal/provider/rssblog"
	_ "github.com/harrisonwang/sift/internal/provider/twitter"
)
