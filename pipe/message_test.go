package pipe_test

// func TestSimpleParams(t *testing.T) {
// 	p := &mock.Pump{}
// 	interval := 10 * time.Millisecond
// 	params := pipe.NewParams(p.IntervalParam(interval))
// 	params.ApplyTo(p.ID())

// 	assert.Equal(t, interval, p.Interval)
// }

// func TestMergeParams(t *testing.T) {
// 	var params *pipe.Params
// 	p := &mock.Pump{}

// 	interval := 10 * time.Millisecond
// 	newParams := pipe.NewParams(p.IntervalParam(interval))
// 	params = params.Merge(newParams)
// 	params.ApplyTo(p.ID())
// 	assert.Equal(t, interval, p.Interval)

// 	newInterval := 20 * time.Millisecond
// 	newParams = pipe.NewParams(p.IntervalParam(newInterval))
// 	params = params.Merge(newParams)
// 	params.ApplyTo(p.ID())
// 	assert.Equal(t, newInterval, p.Interval)
// }
