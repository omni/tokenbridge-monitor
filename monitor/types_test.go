package monitor_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/omni/tokenbridge-monitor/entity"
	"github.com/omni/tokenbridge-monitor/monitor"
)

func TestSplitBlockRange(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Name           string
		Input          [3]uint
		ExpectedOutput []*monitor.BlocksRange
	}{
		{
			Name:  "Split range in two",
			Input: [3]uint{100, 199, 50},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 149},
				{150, 199},
			},
		},
		{
			Name:  "Split range in two 2",
			Input: [3]uint{100, 200, 90},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 189},
				{190, 200},
			},
		},
		{
			Name:  "Split range in three",
			Input: [3]uint{100, 200, 50},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 149},
				{150, 199},
				{200, 200},
			},
		},
		{
			Name:  "Keep range as is",
			Input: [3]uint{100, 200, 101},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 200},
			},
		},
		{
			Name:  "Keep range as is 2",
			Input: [3]uint{100, 200, 999},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 200},
			},
		},
		{
			Name:  "Keep range of one block",
			Input: [3]uint{100, 100, 10},
			ExpectedOutput: []*monitor.BlocksRange{
				{100, 100},
			},
		},
		{
			Name:  "Split range in many subranges",
			Input: [3]uint{100000, 201000, 5000},
			ExpectedOutput: []*monitor.BlocksRange{
				{100000, 104999},
				{105000, 109999},
				{110000, 114999},
				{115000, 119999},
				{120000, 124999},
				{125000, 129999},
				{130000, 134999},
				{135000, 139999},
				{140000, 144999},
				{145000, 149999},
				{150000, 154999},
				{155000, 159999},
				{160000, 164999},
				{165000, 169999},
				{170000, 174999},
				{175000, 179999},
				{180000, 184999},
				{185000, 189999},
				{190000, 194999},
				{195000, 199999},
				{200000, 201000},
			},
		},
		{
			Name:           "Invalid range",
			Input:          [3]uint{200, 100, 50},
			ExpectedOutput: []*monitor.BlocksRange{},
		},
		{
			Name:           "Invalid range 2",
			Input:          [3]uint{200, 100, 500},
			ExpectedOutput: []*monitor.BlocksRange{},
		},
	} {
		t.Logf("Running sub-test %q", test.Name)
		res := monitor.SplitBlockRange(test.Input[0], test.Input[1], test.Input[2])
		require.Equal(t, test.ExpectedOutput, res, "Failed %s", test.Name)
	}
}

func TestSplitLogsInBatches(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Name           string
		Input          []*entity.Log
		ExpectedOutput []*monitor.LogsBatch
	}{
		{
			Name: "Split range in two",
			Input: []*entity.Log{
				{ID: 1, BlockNumber: 100},
				{ID: 2, BlockNumber: 100},
				{ID: 3, BlockNumber: 150},
				{ID: 4, BlockNumber: 150},
			},
			ExpectedOutput: []*monitor.LogsBatch{
				{100, []*entity.Log{
					{ID: 1, BlockNumber: 100},
					{ID: 2, BlockNumber: 100},
				}},
				{150, []*entity.Log{
					{ID: 3, BlockNumber: 150},
					{ID: 4, BlockNumber: 150},
				}},
			},
		},
		{
			Name: "Split range in three",
			Input: []*entity.Log{
				{ID: 1, BlockNumber: 100},
				{ID: 2, BlockNumber: 120},
				{ID: 3, BlockNumber: 120},
				{ID: 4, BlockNumber: 150},
			},
			ExpectedOutput: []*monitor.LogsBatch{
				{100, []*entity.Log{
					{ID: 1, BlockNumber: 100},
				}},
				{120, []*entity.Log{
					{ID: 2, BlockNumber: 120},
					{ID: 3, BlockNumber: 120},
				}},
				{150, []*entity.Log{
					{ID: 4, BlockNumber: 150},
				}},
			},
		},
		{
			Name:           "Leave empty range",
			Input:          []*entity.Log{},
			ExpectedOutput: []*monitor.LogsBatch{},
		},
		{
			Name: "Keep single range of one log as is",
			Input: []*entity.Log{
				{ID: 1, BlockNumber: 100},
			},
			ExpectedOutput: []*monitor.LogsBatch{
				{100, []*entity.Log{
					{ID: 1, BlockNumber: 100},
				}},
			},
		},
		{
			Name: "Keep single range as is",
			Input: []*entity.Log{
				{ID: 1, BlockNumber: 100},
				{ID: 2, BlockNumber: 100},
				{ID: 3, BlockNumber: 100},
			},
			ExpectedOutput: []*monitor.LogsBatch{
				{100, []*entity.Log{
					{ID: 1, BlockNumber: 100},
					{ID: 2, BlockNumber: 100},
					{ID: 3, BlockNumber: 100},
				}},
			},
		},
	} {
		t.Logf("Running sub-test %q", test.Name)
		res := monitor.SplitLogsInBatches(test.Input)
		require.Equal(t, test.ExpectedOutput, res, "Failed %s", test.Name)
	}
}
