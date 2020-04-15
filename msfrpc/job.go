package msfrpc

import (
	"context"

	"github.com/pkg/errors"
)

// JobList is used to list current jobs, key = job id and value = job name.
func (msf *MSFRPC) JobList(ctx context.Context) (map[string]string, error) {
	request := JobListRequest{
		Method: MethodJobList,
		Token:  msf.GetToken(),
	}
	var (
		result   map[string]string
		msfError MSFError
	)
	err := msf.sendWithReplace(ctx, &request, &result, &msfError)
	if err != nil {
		return nil, err
	}
	if msfError.Err {
		if msfError.ErrorMessage == ErrInvalidToken {
			msfError.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&msfError)
	}
	return result, nil
}

// JobInfo is used to get additional data about a specific job. This includes the start
// time and complete datastore of the module associated with the job.
func (msf *MSFRPC) JobInfo(ctx context.Context, id string) (*JobInfoResult, error) {
	request := JobInfoRequest{
		Method: MethodJobInfo,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result JobInfoResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidJobID:
			result.ErrorMessage = ErrInvalidJobIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	// replace []byte to string
	for key, value := range result.DataStore {
		if v, ok := value.([]byte); ok {
			result.DataStore[key] = string(v)
		}
	}
	return &result, nil
}

// JobStop is used to terminate the job specified by the Job ID.
func (msf *MSFRPC) JobStop(ctx context.Context, id string) error {
	request := JobStopRequest{
		Method: MethodJobStop,
		Token:  msf.GetToken(),
		ID:     id,
	}
	var result JobStopResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		switch result.ErrorMessage {
		case ErrInvalidJobID:
			result.ErrorMessage = ErrInvalidJobIDPrefix + id
		case ErrInvalidToken:
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}
