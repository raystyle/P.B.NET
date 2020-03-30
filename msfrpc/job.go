package msfrpc

// JobList is used to list current jobs, key = job id and  value = job name.
func (msf *MSFRPC) JobList() (map[string]string, error) {
	request := JobListRequest{
		Method: MethodJobList,
		Token:  msf.GetToken(),
	}
	var (
		result   map[string]string
		msfError MSFError
	)
	err := msf.sendWithReplace(msf.ctx, &request, &result, &msfError)
	if err != nil {
		return nil, err
	}
	if msfError.Err {
		return nil, &msfError
	}
	return result, nil
}
