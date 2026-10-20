package workspace

// Reserved lists, when asked, the folders hearth keeps for itself. It is a
// function so a folder the owner names while the core runs, such as the
// templates in a Hearth folder only just chosen, counts at the next check
// rather than at the next launch. A nil Reserved lists none.
type Reserved func() []string

// Folders is the current list.
func (r Reserved) Folders() []string {
	if r == nil {
		return nil
	}
	return r()
}
