package demo

// GreeterIDL is the io.vlx.demo.Greeter interface.
const GreeterIDL = `# A friendly greeter shipped with vlx, so you can explore Varlink without
# systemd. Start it with 'vlx demo'.
interface io.vlx.demo.Greeter

# The languages the greeter speaks.
type Language (english, spanish, german, japanese)

# Returns a greeting for someone.
method GetGreeting(
	# Who to greet.
	name: string,
	# The language to greet in. Uses the default language when unset.
	language: ?Language
) -> (
	# The greeting, ready to print.
	greeting: string,
	language: Language
)

# Lists every language the greeter knows, and which one is the default.
method ListLanguages() -> (
	languages: []Language,
	default: Language
)

# Counts down to zero, sending one reply per tick. Press Esc in vlx to stop
# it early.
# [Requires 'more' flag]
method WatchCountdown(
	# Where to start counting, at most 100.
	from: int,
	# Milliseconds between replies, 50 to 5000. Defaults to 500.
	intervalMilliseconds: ?int
) -> (
	remaining: int
)

# Changes the default language. This changes state, so vlx only sends it
# with --allow-writes and asks first.
method SetDefaultLanguage(language: Language) -> ()

# The name was empty.
error EmptyName ()
`

// InventoryIDL is the io.vlx.demo.Inventory interface.
const InventoryIDL = `# A tiny in-memory warehouse. It shows nested types, maps, optional fields,
# streaming replies, typed errors and methods of every safety class.
interface io.vlx.demo.Inventory

# Physical size in centimetres.
type Dimensions (
	width: float,
	height: float,
	depth: float
)

# One stock item.
type Item (
	id: int,
	name: string,
	quantity: int,
	tags: []string,
	# Not every item has been measured.
	dimensions: ?Dimensions,
	# Free-form attributes.
	attributes: [string]string
)

# Who supplies an item. The token is secret, so vlx redacts it.
type Supplier (
	name: string,
	email: string,
	apiToken: string
)

# Returns one item by id.
method GetItem(id: int) -> (item: Item)

# Lists items, one reply per item, optionally only those with a tag.
# [Supports 'more' flag]
method ListItems(tag: ?string) -> (item: Item)

# Returns the supplier of an item.
method GetSupplier(id: int) -> (supplier: Supplier)

# Adds an item and returns it with its new id.
method AddItem(
	name: string,
	quantity: int,
	tags: ?[]string,
	attributes: ?[string]string
) -> (item: Item)

# Removes an item. vlx treats this as dangerous and asks you to type the
# method name before sending it.
method RemoveItem(id: int) -> ()

# Restores the initial stock.
method Reset() -> ()

# No item has this id.
error NoSuchItem (id: int)
`

// ServiceIDL is org.varlink.service, the introspection interface every
// Varlink service implements.
const ServiceIDL = `# The Varlink Service Interface is provided by every varlink service. It
# describes the service and the interfaces it implements.
interface org.varlink.service

# Get a list of all the interfaces a service provides and information
# about the implementation.
method GetInfo() -> (
	vendor: string,
	product: string,
	version: string,
	url: string,
	interfaces: []string
)

# Get the description of an interface that is implemented by this service.
method GetInterfaceDescription(interface: string) -> (description: string)

# The requested interface was not found.
error InterfaceNotFound (interface: string)

# The requested method was not found
error MethodNotFound (method: string)

# The interface defines the requested method, but the service does not
# implement it.
error MethodNotImplemented (method: string)

# One of the passed parameters is invalid.
error InvalidParameter (parameter: string)

# Client is denied access
error PermissionDenied ()

# Method is expected to be called with 'more' set to true, but wasn't
error ExpectedMore ()
`
