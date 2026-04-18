package formal

import "fmt"

// AtLeastNHealthy returns an Invariant that requires at least n services to be Healthy=true.
func AtLeastNHealthy(n int) Invariant {
	return Invariant{
		Name:        fmt.Sprintf("at-least-%d-healthy", n),
		Description: fmt.Sprintf("at least %d service(s) must be healthy", n),
		Fn: func(w World) bool {
			count := 0
			for _, s := range w {
				if s.Healthy {
					count++
				}
			}
			return count >= n
		},
	}
}

// NoMoreThanKUnhealthy returns an Invariant allowing at most k services to be Healthy=false.
func NoMoreThanKUnhealthy(k int) Invariant {
	return Invariant{
		Name:        fmt.Sprintf("no-more-than-%d-unhealthy", k),
		Description: fmt.Sprintf("at most %d service(s) may be unhealthy", k),
		Fn: func(w World) bool {
			count := 0
			for _, s := range w {
				if !s.Healthy {
					count++
				}
			}
			return count <= k
		},
	}
}

// ServiceAlwaysHealthy returns an Invariant requiring a named service to always be Healthy=true.
func ServiceAlwaysHealthy(service string) Invariant {
	return Invariant{
		Name:        fmt.Sprintf("service-%s-always-healthy", service),
		Description: fmt.Sprintf("service %q must always be healthy", service),
		Fn: func(w World) bool {
			s, ok := w[service]
			if !ok {
				return true // absent service is not tracked; no violation
			}
			return s.Healthy
		},
	}
}

// MinReplicas returns an Invariant requiring a named service to always have Replicas >= n.
func MinReplicas(service string, n int) Invariant {
	return Invariant{
		Name:        fmt.Sprintf("min-replicas-%s-%d", service, n),
		Description: fmt.Sprintf("service %q must always have at least %d replica(s)", service, n),
		Fn: func(w World) bool {
			s, ok := w[service]
			if !ok {
				return true // absent service; no violation
			}
			return s.Replicas >= n
		},
	}
}

// Conjunction combines multiple invariants into one that requires ALL to hold.
func Conjunction(name string, invs ...Invariant) Invariant {
	return Invariant{
		Name:        name,
		Description: fmt.Sprintf("conjunction of %d invariants", len(invs)),
		Fn: func(w World) bool {
			for _, inv := range invs {
				if !inv.Fn(w) {
					return false
				}
			}
			return true
		},
	}
}

// Disjunction combines multiple invariants into one that requires AT LEAST ONE to hold.
func Disjunction(name string, invs ...Invariant) Invariant {
	return Invariant{
		Name:        name,
		Description: fmt.Sprintf("disjunction of %d invariants", len(invs)),
		Fn: func(w World) bool {
			for _, inv := range invs {
				if inv.Fn(w) {
					return true
				}
			}
			return false
		},
	}
}

// Negation returns the logical NOT of an invariant.
func Negation(inv Invariant) Invariant {
	return Invariant{
		Name:        fmt.Sprintf("not-%s", inv.Name),
		Description: fmt.Sprintf("negation of %q", inv.Name),
		Fn: func(w World) bool {
			return !inv.Fn(w)
		},
	}
}
