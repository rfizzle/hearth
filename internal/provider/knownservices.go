package provider

// KnownKind is what sort of service a known service is.
type KnownKind string

// The kinds of known service.
const (
	// Builtin is one of the providers with an account of its own; the id
	// is its Provider.
	Builtin KnownKind = "builtin"
	// CompatibleService is an OpenAI-compatible service on the internet; it
	// takes a key.
	CompatibleService KnownKind = "compatible"
	// Local is an OpenAI-compatible runtime on this Mac; it takes no key.
	Local KnownKind = "local"
)

// KnownService is a prefill for adding a service: where it is, where its
// key comes from, and a model it serves. It is data, not code: the person
// adds any of them, or a gateway that is not here, through SetProfile.
type KnownService struct {
	ID      string
	Name    string
	BaseURL string
	HelpURL string
	// DefaultModel is offered as the profile's model; empty for a builtin,
	// which has its own defaults.
	DefaultModel string
	Kind         KnownKind
}

// knownServices is every service hearth knows the address of. Another one
// is a row here. The builtins are listed so a search for one in the app's
// list finds its card. The rows are positional, in field order, so the
// table reads as data: id, name, address, help page, model, kind.
var knownServices = []KnownService{
	{"zai", "Z.ai", "https://api.z.ai/api/paas/v4", "https://z.ai/manage-apikey/apikey-list", "glm-4.6", "compatible"},
	{"groq", "Groq", "https://api.groq.com/openai/v1", "https://console.groq.com/keys", "llama-3.3-70b-versatile", "compatible"},
	{"deepseek", "DeepSeek", "https://api.deepseek.com/v1", "https://platform.deepseek.com/api_keys", "deepseek-chat", "compatible"},
	{"mistral", "Mistral", "https://api.mistral.ai/v1", "https://console.mistral.ai/api-keys", "mistral-large-latest", "compatible"},
	{"together", "Together", "https://api.together.xyz/v1", "https://api.together.ai/settings/api-keys", "meta-llama/Llama-3.3-70B-Instruct-Turbo", "compatible"},
	{"ollama", "Ollama (local)", "http://localhost:11434/v1", "https://ollama.com/download", "llama3.2", "local"},
	{"lmstudio", "LM Studio (local)", "http://localhost:1234/v1", "https://lmstudio.ai", "local-model", "local"},
	{"chatgpt", "ChatGPT", "https://chatgpt.com/backend-api/codex", "https://chatgpt.com", "", "builtin"},
	{"anthropic", "Anthropic", "https://api.anthropic.com", "https://console.anthropic.com/settings/keys", "", "builtin"},
	{"gemini", "Gemini", "https://generativelanguage.googleapis.com", "https://aistudio.google.com/apikey", "", "builtin"},
	{"openrouter", "OpenRouter", "https://openrouter.ai/api/v1", "https://openrouter.ai/keys", "", "builtin"},
}

// KnownServices lists the known services in table order.
func KnownServices() []KnownService {
	return append([]KnownService(nil), knownServices...)
}
