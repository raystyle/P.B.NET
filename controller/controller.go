package controller

type CONTROLLER struct {
	config   *Config
	database *database
	logger   *ctrl_logger
	global   *global
}

func New(c *Config) (*CONTROLLER, error) {
	ctrl := &CONTROLLER{config: c}
	db, err := new_database(ctrl)
	if err != nil {
		return nil, err
	}
	ctrl.database = db
	return ctrl, nil
}

func (this *CONTROLLER) Main() error {
	err := this.database.Connect()
	if err != nil {
		return err
	}
	l, err := new_ctrl_logger(this)
	if err != nil {
		return err
	}
	this.logger = l
	g, err := new_global(this)
	if err != nil {
		return err
	}
	this.global = g
	return nil
}

func (this *CONTROLLER) Exit() error {

	return nil
}
