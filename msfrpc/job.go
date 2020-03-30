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

// JobInfo is used to get additional data about a specific job. This includes the start
// time and complete datastore of the module associated with the job.
func (msf *MSFRPC) JobInfo(id string) (*JobInfoResult, error) {
	request := JobInfoRequest{
		Method: MethodJobInfo,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result JobInfoResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		return nil, &result.MSFError
	}
	// replace []byte to string
	for key, value := range result.DataStore {
		if v, ok := value.([]byte); ok {
			result.DataStore[key] = string(v)
		}
	}
	return &result, nil
}
