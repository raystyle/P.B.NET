package controller

type CONTROLLER struct {
	config   *Config
	database *database
	global   *global
}

func New(c *Config) (*CONTROLLER, error) {
	ctrl := &CONTROLLER{config: c}
	db, err := new_database(ctrl)
	if err != nil {
		return nil, err
	}
	ctrl.database = db
	g, err := new_global(ctrl)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	return ctrl, nil
}

func (this *CONTROLLER) Main() error {
	err := this.database.Connect()
	if err != nil {
		return err
	}
	return nil
}

func (this *CONTROLLER) Exit() error {

	return nil
}
