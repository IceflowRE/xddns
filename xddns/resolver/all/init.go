package all

import (
	// Importing all available resolvers.
	_ "github.com/iceflowre/xddns/xddns/resolver/ipservice"
	_ "github.com/iceflowre/xddns/xddns/resolver/netif"
	_ "github.com/iceflowre/xddns/xddns/resolver/shell"
)
