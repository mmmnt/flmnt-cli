package sync

// Link is `flmnt link` (QRM-ADR-016 §5d, SPEC-LINK-001): the rows for this machine's identity and the
// subs it replaced, written to the hosted account the endpoint's token belongs to, and no records —
// the recovery for a teammate who writes to a host's harness and never pushes.
const linkMutation = `mutation($localSubs: [String!]!){ linkLocalIdentity(localSubs: $localSubs){ linked held } }`

// LinkReport is what the server answered: the subs linked now and the subs already the account's.
type LinkReport struct {
	Linked []string `json:"linked"`
	Held   []string `json:"held"`
}

func Link(ep Endpoint, localSubs []string) (LinkReport, error) {
	var data struct {
		LinkLocalIdentity LinkReport `json:"linkLocalIdentity"`
	}
	if err := ep.GQL.Query(linkMutation, map[string]any{"localSubs": localSubs}, &data); err != nil {
		return LinkReport{}, err
	}
	return data.LinkLocalIdentity, nil
}
