// +build windows

package wmi

import (
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/pkg/errors"
)

// Object returned by Client.Get().
type Object struct {
	raw *ole.VARIANT
}

// count is used to get the number of objects.
func (obj *Object) count() (int, error) {
	iDispatch := obj.raw.ToIDispatch()
	if iDispatch == nil {
		return 0, nil
	}
	iDispatch.AddRef()
	defer iDispatch.Release()
	prop, err := oleutil.GetProperty(iDispatch, "Count")
	if err != nil {
		return 0, errors.Wrap(err, "failed to get Count property")
	}
	return int(prop.Val), nil
}

// need clear object.
func (obj *Object) itemIndex(i int) (*Object, error) {
	iDispatch := obj.raw.ToIDispatch()
	if iDispatch == nil {
		return nil, errors.New("object is not callable")
	}
	iDispatch.AddRef()
	defer iDispatch.Release()
	itemRaw, err := oleutil.CallMethod(iDispatch, "ItemIndex", i)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call ItemIndex")
	}
	return &Object{raw: itemRaw}, nil
}

// need clear each object, after use.
func (obj *Object) objects() ([]*Object, error) {
	count, err := obj.count()
	if err != nil {
		return nil, err
	}
	objects := make([]*Object, count)
	for i := 0; i < count; i++ {
		objects[i], err = obj.itemIndex(i)
		if err != nil {
			// clear objects
			for j := 0; j < i; j++ {
				objects[j].Clear()
			}
			return nil, err
		}
	}
	return objects, nil
}

// ExecMethod is used to execute a method on the object.
func (obj *Object) ExecMethod(method string, args ...interface{}) (*Object, error) {
	iDispatch := obj.raw.ToIDispatch()
	if iDispatch == nil {
		return nil, errors.New("object is not callable")
	}
	iDispatch.AddRef()
	defer iDispatch.Release()
	returnValue, err := oleutil.CallMethod(iDispatch, method, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call method %q", method)
	}
	return &Object{raw: returnValue}, nil
}

// GetProperty is used to get property of this object, need clear object.
func (obj *Object) GetProperty(property string) (*Object, error) {
	iDispatch := obj.raw.ToIDispatch()
	if iDispatch == nil {
		return nil, errors.New("object is not callable")
	}
	iDispatch.AddRef()
	defer iDispatch.Release()
	prop, err := oleutil.GetProperty(iDispatch, property)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get property %q", property)
	}
	return &Object{raw: prop}, nil
}

// SetProperty is used to set property of this object.
func (obj *Object) SetProperty(property string, args ...interface{}) error {
	iDispatch := obj.raw.ToIDispatch()
	if iDispatch == nil {
		return errors.New("object is not callable")
	}
	iDispatch.AddRef()
	defer iDispatch.Release()
	_, err := oleutil.PutProperty(iDispatch, property, args...)
	if err != nil {
		return errors.Wrapf(err, "failed to set property %q", property)
	}
	return nil
}

// GetMethodInputParameters is used to get input parameters about a method.
func (obj *Object) GetMethodInputParameters(name string) (*Object, error) {
	methods, err := obj.GetProperty("Methods_")
	if err != nil {
		return nil, err
	}
	defer methods.Clear()
	method, err := methods.ExecMethod("Item", name)
	if err != nil {
		return nil, err
	}
	defer method.Clear()
	input, err := method.GetProperty("InParameters")
	if err != nil {
		return nil, err
	}
	return input, nil
}

// Path is used to get path about this object.
func (obj *Object) Path() (string, error) {
	prop, err := obj.GetProperty("Path_")
	if err != nil {
		return "", err
	}
	defer prop.Clear()
	path, err := prop.GetProperty("Path")
	if err != nil {
		return "", err
	}
	defer path.Clear()
	return path.Value().(string), nil
}

// Value is used to return the value of a result as an interface.
func (obj *Object) Value() interface{} {
	if obj == nil || obj.raw == nil {
		return ""
	}
	return obj.raw.Value()
}

// ToArray is used to return a *ole.SafeArrayConversion from the WMI result.
func (obj *Object) ToArray() *ole.SafeArrayConversion {
	if obj == nil || obj.raw == nil {
		return nil
	}
	return obj.raw.ToArray()
}

// Clear is used to clear the memory of variant object.
func (obj *Object) Clear() {
	_ = obj.raw.Clear()
}
