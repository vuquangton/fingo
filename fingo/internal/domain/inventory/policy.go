package inventory

import (
	"fmt"
	"sort"
)

// Method is how the cost of goods sold is worked out.
//
// TT133 (TK 152 khoản 1.c, and again for thành phẩm at TK 155) permits exactly
// three, and names the retail method in the accounting-policy note:
//
//	"Việc tính trị giá của nguyên liệu, vật liệu xuất kho trong kỳ, được thực
//	 hiện theo một trong các phương pháp sau:
//	 - Phương pháp giá thực tế đích danh;
//	 - Phương pháp bình quân gia quyền sau mỗi lần nhập hoặc cuối kỳ;
//	 - Phương pháp nhập trước, xuất trước."
type Method string

const (
	// MethodSpecificCost is giá thực tế đích danh: the actual cost of the units
	// that left, which requires knowing which ones they were.
	MethodSpecificCost Method = "SPECIFIC_COST"

	// MethodMovingAverage is bình quân gia quyền sau mỗi lần nhập: the average
	// is recomputed on every receipt.
	MethodMovingAverage Method = "MOVING_AVERAGE"

	// MethodPeriodicAverage is bình quân gia quyền cuối kỳ: one average for the
	// whole period, worked out at its end.
	MethodPeriodicAverage Method = "PERIODIC_AVERAGE"

	// MethodFIFO is nhập trước, xuất trước.
	MethodFIFO Method = "FIFO"
)

func (m Method) Valid() bool {
	switch m {
	case MethodSpecificCost, MethodMovingAverage, MethodPeriodicAverage, MethodFIFO:
		return true
	}
	return false
}

// Policy records the valuation method a client applies, by fiscal year.
//
// TT133 requires the chosen method to be consistent for a whole fiscal year:
//
//	"Doanh nghiệp lựa chọn phương pháp tính giá nào thì phải đảm bảo tính nhất
//	 quán trong cả niên độ kế toán."
//
// So the method is held per year rather than as one setting, and changing it
// within a year is refused. A mid-year change would silently restate a cost of
// sales that has already been reported, and nothing downstream could tell that
// the two halves of the year were measured differently.
type Policy struct {
	byYear map[int]Method
}

func NewPolicy() *Policy {
	return &Policy{byYear: make(map[int]Method)}
}

// Set chooses the method for a fiscal year.
//
// Setting the same method again is allowed — it is the same decision. Setting a
// different one for a year that already has a method is refused.
func (p *Policy) Set(year int, method Method) error {
	if year < 1 {
		return fmt.Errorf("a valuation policy needs a fiscal year, got %d", year)
	}
	if !method.Valid() {
		return fmt.Errorf("valuation method %q is not one of the methods TT133 permits", method)
	}

	if existing, chosen := p.byYear[year]; chosen && existing != method {
		return fmt.Errorf(
			"fiscal year %d is valued by %s and cannot be changed to %s: TT133 requires one method for the whole year",
			year, existing, method)
	}

	p.byYear[year] = method

	return nil
}

// MethodFor returns the method a fiscal year is valued by.
//
// An unset year is an error rather than a default: a default would value a
// client's stock by a method they never chose, and the choice has to appear in
// their accounting-policy note.
func (p *Policy) MethodFor(year int) (Method, error) {
	method, chosen := p.byYear[year]
	if !chosen {
		return "", fmt.Errorf("no valuation method has been chosen for fiscal year %d", year)
	}

	return method, nil
}

// Years returns the fiscal years that have a method, oldest first.
func (p *Policy) Years() []int {
	out := make([]int, 0, len(p.byYear))
	for year := range p.byYear {
		out = append(out, year)
	}
	sort.Ints(out)

	return out
}
