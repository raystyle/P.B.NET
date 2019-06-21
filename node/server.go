package node

type server struct {
	ctx       *NODE
	listeners map[string]*listener
}

func new_server(ctx *NODE) (*server, error) {
	l := &server{
		ctx: ctx,
	}
	return l, nil
}

func (this *server) Kill(tag string) {

}

func (this *server) Shutdown() {

}

type listener struct {
}
