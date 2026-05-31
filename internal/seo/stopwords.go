package seo

// stopwords are common English words excluded from keyword-density analysis.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
	"you": true, "all": true, "any": true, "can": true, "had": true, "her": true,
	"was": true, "one": true, "our": true, "out": true, "his": true, "has": true,
	"how": true, "man": true, "new": true, "now": true, "old": true, "see": true,
	"two": true, "way": true, "who": true, "boy": true, "did": true, "its": true,
	"let": true, "put": true, "say": true, "she": true, "too": true, "use": true,
	"that": true, "with": true, "this": true, "have": true, "from": true, "they": true,
	"will": true, "would": true, "there": true, "their": true, "what": true,
	"about": true, "which": true, "when": true, "make": true, "like": true,
	"time": true, "just": true, "him": true, "know": true, "take": true,
	"into": true, "your": true, "some": true, "could": true, "them": true,
	"than": true, "then": true, "look": true, "only": true, "come": true,
	"over": true, "also": true, "back": true, "after": true, "where": true,
	"because": true, "these": true, "those": true, "such": true, "being": true,
	"been": true, "were": true, "does": true, "doing": true, "here": true,
	"more": true, "most": true, "other": true, "should": true, "very": true,
	"each": true, "while": true, "between": true, "both": true, "through": true,
	"during": true, "before": true, "above": true, "below": true, "again": true,
	"once": true, "under": true, "same": true, "down": true, "off": true,
	"using": true, "used": true, "via": true, "per": true, "etc": true,
}
