package server

type GRPCGWConfig struct {
	GRPCPort   string
	GRPCGWPort string
}

// nolint
type GRPCGW struct {
	config *GRPCGWConfig
}

func ListenGRPCGW(config *GRPCGWConfig) error { //conn, err := grpc.Dial(config.GRPCPort, grpc.WithInsecure())
	//if err != nil {
	//	return err
	//}
	//mux := runtime.NewServeMux()
	//ctx := context.TODO()
	//if err := pb.RegisterScenarioServiceHandler(ctx, mux, conn); err != nil {
	//	return err
	//}
	//h := http.Handler(mux)
	//err = http.ListenAndServe(config.GRPCGWPort, h)
	//if err != nil && err != http.ErrServerClosed {
	//	return err
	//}
	return nil
}
