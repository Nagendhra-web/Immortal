package pagination_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/pagination"
)

func TestPaginate(t *testing.T) {
	p := pagination.Paginate(100, 1, 20)
	if p.Offset != 0 {
		t.Errorf("expected offset 0, got %d", p.Offset)
	}
	if p.Limit != 20 {
		t.Errorf("expected limit 20, got %d", p.Limit)
	}
}

func TestPaginatePage2(t *testing.T) {
	p := pagination.Paginate(100, 2, 20)
	if p.Offset != 20 {
		t.Errorf("expected offset 20, got %d", p.Offset)
	}
}

func TestPaginateLastPage(t *testing.T) {
	p := pagination.Paginate(95, 5, 20)
	if p.Limit != 15 {
		t.Errorf("expected limit 15, got %d", p.Limit)
	}
}

func TestPaginateInvalidPage(t *testing.T) {
	p := pagination.Paginate(100, 0, 20)
	if p.Offset != 0 {
		t.Error("invalid page should default to 1")
	}
}

func TestResponse(t *testing.T) {
	r := pagination.NewResponse([]string{"a", "b"}, 50, 1, 20)
	if !r.HasNext {
		t.Error("should have next page")
	}
	if r.HasPrev {
		t.Error("page 1 should not have prev")
	}
	if r.TotalPages != 3 {
		t.Errorf("expected 3 pages, got %d", r.TotalPages)
	}
}

func TestResponseLastPage(t *testing.T) {
	r := pagination.NewResponse(nil, 50, 3, 20)
	if r.HasNext {
		t.Error("last page should not have next")
	}
	if !r.HasPrev {
		t.Error("page 3 should have prev")
	}
}
