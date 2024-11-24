package ctrl

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Configuration are the structure that holds all the different
// configuration options used both with flags and the config file.
// If a new field is added to this struct there should also be
// added the same field to the ConfigurationFromFile struct, and
// an if check should be added to the checkConfigValues function
// to set default values when reading from config file.
type Configuration struct {
	// ConfigFolder, the location for the configuration folder on disk
	ConfigFolder string `comment:"ConfigFolder, the location for the configuration folder on disk"`
	// The folder where the socket file should live
	SocketFolder string `comment:"The folder where the socket file should live"`
	// The folder where the readfolder should live
	ReadFolder string `comment:"The folder where the readfolder should live"`
	// EnableReadFolder for enabling the read messages api from readfolder
	EnableReadFolder bool `comment:"EnableReadFolder for enabling the read messages api from readfolder"`
	// TCP Listener for sending messages to the system, <host>:<port>
	TCPListener string `comment:"TCP Listener for sending messages to the system, <host>:<port>"`
	// HTTP Listener for sending messages to the system, <host>:<port>
	HTTPListener string `comment:"HTTP Listener for sending messages to the system, <host>:<port>"`
	// The folder where the database should live
	DatabaseFolder string `comment:"The folder where the database should live"`
	// Unique string to identify this Edge unit
	NodeName string `comment:"Unique string to identify this Edge unit"`
	// The address of the message broker, <address>:<port>
	BrokerAddress string `comment:"The address of the message broker, <address>:<port>"`
	// NatsConnOptTimeout the timeout for trying the connect to nats broker
	NatsConnOptTimeout int `comment:"NatsConnOptTimeout the timeout for trying the connect to nats broker"`
	// Nats connect retry interval in seconds
	NatsConnectRetryInterval int `comment:"Nats connect retry interval in seconds"`
	// NatsReconnectJitter in milliseconds
	NatsReconnectJitter int `comment:"NatsReconnectJitter in milliseconds"`
	// NatsReconnectJitterTLS in seconds
	NatsReconnectJitterTLS int `comment:"NatsReconnectJitterTLS in seconds"`
	// KeysUpdateInterval in seconds
	KeysUpdateInterval int `comment:"KeysUpdateInterval in seconds"`
	// AclUpdateInterval in seconds
	AclUpdateInterval int `comment:"AclUpdateInterval in seconds"`
	// The number of the profiling port
	ProfilingPort string `comment:"The number of the profiling port"`
	// Host and port for prometheus listener, e.g. localhost:2112
	PromHostAndPort string `comment:"Host and port for prometheus listener, e.g. localhost:2112"`
	// Set to true if this is the node that should receive the error log's from other nodes
	DefaultMessageTimeout int `comment:"Set to true if this is the node that should receive the error log's from other nodes"`
	// Default value for how long can a request method max be allowed to run in seconds
	DefaultMethodTimeout int `comment:"Default value for how long can a request method max be allowed to run in seconds"`
	// Default amount of retries that will be done before a message is thrown away, and out of the system
	DefaultMessageRetries int `comment:"Default amount of retries that will be done before a message is thrown away, and out of the system"`
	// The path to the data folder
	SubscribersDataFolder string `comment:"The path to the data folder"`
	// Name of central node to receive logs, errors, key/acl handling
	CentralNodeName string `comment:"Name of central node to receive logs, errors, key/acl handling"`
	// The full path to the certificate of the root CA
	RootCAPath string `comment:"The full path to the certificate of the root CA"`
	// Full path to the NKEY's seed file
	NkeySeedFile string `comment:"Full path to the NKEY's seed file"`
	// The full path to the NKEY user file
	NkeyPublicKey string `toml:"-"`
	//
	NkeyFromED25519SSHKeyFile string `comment:"Full path to the ED25519 SSH private key. Will generate the NKEY Seed from an SSH ED25519 private key file. NB: This option will take precedence over NkeySeedFile if specified"`
	// NkeySeed
	NkeySeed string `toml:"-"`
	// The host and port to expose the data folder, <host>:<port>
	ExposeDataFolder string `comment:"The host and port to expose the data folder, <host>:<port>"`
	// Timeout in seconds for error messages
	ErrorMessageTimeout int `comment:"Timeout in seconds for error messages"`
	// Retries for error messages
	ErrorMessageRetries int `comment:"Retries for error messages"`
	// SetBlockProfileRate for block profiling
	SetBlockProfileRate int `comment:"SetBlockProfileRate for block profiling"`
	// EnableSocket for enabling the creation of a ctrl.sock file
	EnableSocket bool `comment:"EnableSocket for enabling the creation of a ctrl.sock file"`
	// EnableSignatureCheck to enable signature checking
	EnableSignatureCheck bool `comment:"EnableSignatureCheck to enable signature checking"`
	// EnableAclCheck to enable ACL checking
	EnableAclCheck bool `comment:"EnableAclCheck to enable ACL checking"`
	// LogLevel
	LogLevel             string `comment:"LogLevel error/info/warning/debug/none."`
	LogConsoleTimestamps bool   `comment:"LogConsoleTimestamps true/false for enabling or disabling timestamps when printing errors and information to stderr"`
	// KeepPublishersAliveFor number of seconds
	// Timer that will be used for when to remove the sub process
	// publisher. The timer is reset each time a message is published with
	// the process, so the sub process publisher will not be removed until
	// it have not received any messages for the given amount of time.
	KeepPublishersAliveFor int `comment:"KeepPublishersAliveFor number of seconds Timer that will be used for when to remove the sub process publisher. The timer is reset each time a message is published with the process, so the sub process publisher will not be removed until it have not received any messages for the given amount of time."`

	// Comma separated list of additional streams to consume from.
	JetstreamsConsume string

	StartPubHello    int  `comment:"StartPubHello, sets the interval in seconds for how often we send hello messages to central server"`
	EnableKeyUpdates bool `comment:"Enable the updates of public keys"`

	EnableAclUpdates bool `comment:"Enable the updates of acl's"`

	IsCentralErrorLogger    bool `comment:"Start the central error logger"`
	StartSubHello           bool `comment:"Start subscriber for hello messages"`
	StartSubFileAppend      bool `comment:"Start subscriber for text logging"`
	StartSubFile            bool `comment:"Start subscriber for writing to file"`
	StartSubCopySrc         bool `comment:"Start subscriber for reading files to copy"`
	StartSubCopyDst         bool `comment:"Start subscriber for writing copied files to disk"`
	StartSubCliCommand      bool `comment:"Start subscriber for CLICommand"`
	StartSubConsole         bool `comment:"Start subscriber for Console"`
	StartSubHttpGet         bool `comment:"Start subscriber for HttpGet"`
	StartSubTailFile        bool `comment:"Start subscriber for tailing log files"`
	StartSubCliCommandCont  bool `comment:"Start subscriber for continously delivery of output from cli commands."`
	StartJetstreamPublisher bool `comment:"Start the nats jetstream publisher"`
	StartJetstreamConsumer  bool `comment:"Start the nats jetstream consumer"`

	// IsCentralAuth, enable to make this instance take the role as the central auth server
	IsCentralAuth bool `comment:"IsCentralAuth, enable to make this instance take the role as the central auth server"`
}

// NewConfiguration will return a *Configuration.
func NewConfiguration() *Configuration {
	c := newConfigurationDefaults()

	err := godotenv.Load()
	if err != nil {
		log.Printf("info: no .env file found, will only use env vars or flags: %v\n", err)
	}

	//flag.StringVar(&c.ConfigFolder, "configFolder", fc.ConfigFolder, "Defaults to ./usr/local/ctrl/etc/. *NB* This flag is not used, if your config file are located somwhere else than default set the location in an env variable named CONFIGFOLDER")
	flag.StringVar(&c.SocketFolder, "socketFolder", CheckEnv("SOCKET_FOLDER", c.SocketFolder).(string), "folder who contains the socket file. Defaults to ./tmp/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.ReadFolder, "readFolder", CheckEnv("READ_FOLDER", c.ReadFolder).(string), "folder who contains the readfolder. Defaults to ./readfolder/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.TCPListener, "tcpListener", CheckEnv("TCP_LISTENER", c.TCPListener).(string), "start up a TCP listener in addition to the Unix Socket, to give messages to the system. e.g. localhost:8888. No value means not to start the listener, which is default. NB: You probably don't want to start this on any other interface than localhost")
	flag.StringVar(&c.HTTPListener, "httpListener", CheckEnv("HTTP_LISTENER", c.HTTPListener).(string), "start up a HTTP listener in addition to the Unix Socket, to give messages to the system. e.g. localhost:8888. No value means not to start the listener, which is default. NB: You probably don't want to start this on any other interface than localhost")
	flag.StringVar(&c.DatabaseFolder, "databaseFolder", CheckEnv("DATABASE_FOLDER", c.DatabaseFolder).(string), "folder who contains the database file. Defaults to ./var/lib/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.NodeName, "nodeName", CheckEnv("NODE_NAME", c.NodeName).(string), "some unique string to identify this Edge unit")
	flag.StringVar(&c.BrokerAddress, "brokerAddress", CheckEnv("BROKER_ADDRESS", c.BrokerAddress).(string), "the address of the message broker")
	flag.IntVar(&c.NatsConnOptTimeout, "natsConnOptTimeout", CheckEnv("NATS_CONN_OPT_TIMEOUT", c.NatsConnOptTimeout).(int), "default nats client conn timeout in seconds")
	flag.IntVar(&c.NatsConnectRetryInterval, "natsConnectRetryInterval", CheckEnv("NATS_CONNECT_RETRY_INTERVAL", c.NatsConnectRetryInterval).(int), "default nats retry connect interval in seconds.")
	flag.IntVar(&c.NatsReconnectJitter, "natsReconnectJitter", CheckEnv("NATS_RECONNECT_JITTER", c.NatsReconnectJitter).(int), "default nats ReconnectJitter interval in milliseconds.")
	flag.IntVar(&c.NatsReconnectJitterTLS, "natsReconnectJitterTLS", CheckEnv("NATS_RECONNECT_JITTER_TLS", c.NatsReconnectJitterTLS).(int), "default nats ReconnectJitterTLS interval in seconds.")
	flag.IntVar(&c.KeysUpdateInterval, "keysUpdateInterval", CheckEnv("KEYS_UPDATE_INTERVAL", c.KeysUpdateInterval).(int), "default interval in seconds for asking the central for public keys")
	flag.IntVar(&c.AclUpdateInterval, "aclUpdateInterval", CheckEnv("ACL_UPDATE_INTERVAL", c.AclUpdateInterval).(int), "default interval in seconds for asking the central for acl updates")
	flag.StringVar(&c.ProfilingPort, "profilingPort", CheckEnv("PROFILING_PORT", c.ProfilingPort).(string), "The number of the profiling port")
	flag.StringVar(&c.PromHostAndPort, "promHostAndPort", CheckEnv("PROM_HOST_AND_PORT", c.PromHostAndPort).(string), "host and port for prometheus listener, e.g. localhost:2112")
	flag.IntVar(&c.DefaultMessageTimeout, "defaultMessageTimeout", CheckEnv("DEFAULT_MESSAGE_TIMEOUT", c.DefaultMessageTimeout).(int), "default message timeout in seconds. This can be overridden on the message level")
	flag.IntVar(&c.DefaultMessageRetries, "defaultMessageRetries", CheckEnv("DEFAULT_MESSAGE_RETRIES", c.DefaultMessageRetries).(int), "default amount of retries that will be done before a message is thrown away, and out of the system")
	flag.IntVar(&c.DefaultMethodTimeout, "defaultMethodTimeout", CheckEnv("DEFAULT_METHOD_TIMEOUT", c.DefaultMethodTimeout).(int), "default amount of seconds a request method max will be allowed to run")
	flag.StringVar(&c.SubscribersDataFolder, "subscribersDataFolder", CheckEnv("SUBSCRIBER_DATA_FOLDER", c.SubscribersDataFolder).(string), "The data folder where subscribers are allowed to write their data if needed")
	flag.StringVar(&c.CentralNodeName, "centralNodeName", CheckEnv("CENTRAL_NODE_NAME", c.CentralNodeName).(string), "The name of the central node to receive messages published by this node")
	flag.StringVar(&c.RootCAPath, "rootCAPath", CheckEnv("ROOT_CA_PATH", c.RootCAPath).(string), "If TLS, enter the path for where to find the root CA certificate")
	flag.StringVar(&c.NkeyFromED25519SSHKeyFile, "nkeyFromED25519SSHKeyFile", CheckEnv("NKEY_FROM_ED25519_SSH_KEY_FILE", c.NkeyFromED25519SSHKeyFile).(string), "The full path of the nkeys seed file")
	flag.StringVar(&c.NkeySeedFile, "nkeySeedFile", CheckEnv("NKEY_SEED_FILE", c.NkeySeedFile).(string), "Full path to the ED25519 SSH private key. Will generate the NKEY Seed from an SSH ED25519 private key file. NB: This option will take precedence over NkeySeedFile if specified")
	flag.StringVar(&c.NkeySeed, "nkeySeed", CheckEnv("NKEY_SEED", c.NkeySeed).(string), "The actual nkey seed. To use if not stored in file")
	flag.StringVar(&c.ExposeDataFolder, "exposeDataFolder", CheckEnv("EXPOSE_DATA_FOLDER", c.ExposeDataFolder).(string), "If set the data folder will be exposed on the given host:port. Default value is not exposed at all")
	flag.IntVar(&c.ErrorMessageTimeout, "errorMessageTimeout", CheckEnv("ERROR_MESSAGE_TIMEOUT", c.ErrorMessageTimeout).(int), "The number of seconds to wait for an error message to time out")
	flag.IntVar(&c.ErrorMessageRetries, "errorMessageRetries", CheckEnv("ERROR_MESSAGE_RETRIES", c.ErrorMessageRetries).(int), "The number of if times to retry an error message before we drop it")
	flag.IntVar(&c.SetBlockProfileRate, "setBlockProfileRate", CheckEnv("BLOCK_PROFILE_RATE", c.SetBlockProfileRate).(int), "Enable block profiling by setting the value to f.ex. 1. 0 = disabled")
	flag.BoolVar(&c.EnableSocket, "enableSocket", CheckEnv("ENABLE_SOCKET", c.EnableSocket).(bool), "true/false, for enabling the creation of ctrl.sock file")
	flag.BoolVar(&c.EnableSignatureCheck, "enableSignatureCheck", CheckEnv("ENABLE_SIGNATURE_CHECK", c.EnableSignatureCheck).(bool), "true/false *TESTING* enable signature checking.")
	flag.BoolVar(&c.EnableAclCheck, "enableAclCheck", CheckEnv("ENABLE_ACL_CHECK", c.EnableAclCheck).(bool), "true/false *TESTING* enable Acl checking.")
	flag.BoolVar(&c.IsCentralAuth, "isCentralAuth", CheckEnv("IS_CENTRAL_AUTH", c.IsCentralAuth).(bool), "true/false, *TESTING* is this the central auth server")
	flag.StringVar(&c.LogLevel, "logLevel", CheckEnv("LOG_LEVEL", c.LogLevel).(string), "error/info/warning/debug/none")
	flag.BoolVar(&c.LogConsoleTimestamps, "LogConsoleTimestamps", CheckEnv("LOG_CONSOLE_TIMESTAMPS", c.LogConsoleTimestamps).(bool), "true/false for enabling or disabling timestamps when printing errors and information to stderr")
	flag.IntVar(&c.KeepPublishersAliveFor, "keepPublishersAliveFor", CheckEnv("KEEP_PUBLISHERS_ALIVE_FOR", c.KeepPublishersAliveFor).(int), "The amount of time we allow a publisher to stay alive without receiving any messages to publish")

	flag.StringVar(&c.JetstreamsConsume, "jetstreamsConsume", CheckEnv("JETSTREAMS_CONSUME", c.JetstreamsConsume).(string), "Comma separated list of Jetstrams to consume from")

	// Start of Request publishers/subscribers

	flag.IntVar(&c.StartPubHello, "startPubHello", CheckEnv("START_PUB_HELLO", c.StartPubHello).(int), "Make the current node send hello messages to central at given interval in seconds")

	flag.BoolVar(&c.EnableKeyUpdates, "EnableKeyUpdates", CheckEnv("ENABLE_KEY_UPDATES", c.EnableKeyUpdates).(bool), "true/false")

	flag.BoolVar(&c.EnableAclUpdates, "EnableAclUpdates", CheckEnv("ENABLE_ACL_UPDATES", c.EnableAclUpdates).(bool), "true/false")

	flag.BoolVar(&c.IsCentralErrorLogger, "isCentralErrorLogger", CheckEnv("IS_CENTRAL_ERROR_LOGGER", c.IsCentralErrorLogger).(bool), "true/false")
	flag.BoolVar(&c.StartSubHello, "startSubHello", CheckEnv("START_SUB_HELLO", c.StartSubHello).(bool), "true/false")
	flag.BoolVar(&c.StartSubFileAppend, "startSubFileAppend", CheckEnv("START_SUB_FILE_APPEND", c.StartSubFileAppend).(bool), "true/false")
	flag.BoolVar(&c.StartSubFile, "startSubFile", CheckEnv("START_SUB_FILE", c.StartSubFile).(bool), "true/false")
	flag.BoolVar(&c.StartSubCopySrc, "startSubCopySrc", CheckEnv("START_SUB_COPY_SRC", c.StartSubCopySrc).(bool), "true/false")
	flag.BoolVar(&c.StartSubCopyDst, "startSubCopyDst", CheckEnv("START_SUB_COPY_DST", c.StartSubCopyDst).(bool), "true/false")
	flag.BoolVar(&c.StartSubCliCommand, "startSubCliCommand", CheckEnv("START_SUB_CLI_COMMAND", c.StartSubCliCommand).(bool), "true/false")
	flag.BoolVar(&c.StartSubConsole, "startSubConsole", CheckEnv("START_SUB_CONSOLE", c.StartSubConsole).(bool), "true/false")
	flag.BoolVar(&c.StartSubHttpGet, "startSubHttpGet", CheckEnv("START_SUB_HTTP_GET", c.StartSubHttpGet).(bool), "true/false")
	flag.BoolVar(&c.StartSubTailFile, "startSubTailFile", CheckEnv("START_SUB_TAIL_FILE", c.StartSubTailFile).(bool), "true/false")
	flag.BoolVar(&c.StartSubCliCommandCont, "startSubCliCommandCont", CheckEnv("START_SUB_CLI_COMMAND_CONT", c.StartSubCliCommandCont).(bool), "true/false")

	flag.BoolVar(&c.StartJetstreamPublisher, "startJetstreamPublisher", CheckEnv("START_JETSTREAM_PUBLISHER", c.StartJetstreamPublisher).(bool), "Start the nats jetstream publisher")
	flag.BoolVar(&c.StartJetstreamConsumer, "StartJetstreamConsumer", CheckEnv("START_JETSTREAM_CONSUMER", c.StartJetstreamConsumer).(bool), "Start the nats jetstream consumer")

	// Check that mandatory flag values have been set.
	switch {
	case c.NodeName == "":
		log.Fatalf("error: the nodeName config option or flag cannot be empty, check -help\n")
	case c.CentralNodeName == "":
		// TODO: Check out if we should drop to have this as a mandatory flag?
		log.Fatalf("error: the centralNodeName config option or flag cannot be empty, check -help\n")
	}

	flag.Parse()

	return &c
}

// Get a Configuration struct with the default values set.
func newConfigurationDefaults() Configuration {
	c := Configuration{
		ConfigFolder:              "./etc/",
		SocketFolder:              "./tmp",
		ReadFolder:                "./readfolder",
		EnableReadFolder:          true,
		TCPListener:               "",
		HTTPListener:              "",
		DatabaseFolder:            "./var/lib",
		NodeName:                  "",
		BrokerAddress:             "127.0.0.1:4222",
		NatsConnOptTimeout:        20,
		NatsConnectRetryInterval:  10,
		NatsReconnectJitter:       100,
		NatsReconnectJitterTLS:    1,
		KeysUpdateInterval:        60,
		AclUpdateInterval:         60,
		ProfilingPort:             "",
		PromHostAndPort:           "",
		DefaultMessageTimeout:     10,
		DefaultMessageRetries:     1,
		DefaultMethodTimeout:      10,
		SubscribersDataFolder:     "./data",
		CentralNodeName:           "central",
		RootCAPath:                "",
		NkeySeedFile:              "",
		NkeyFromED25519SSHKeyFile: "",
		NkeySeed:                  "",
		ExposeDataFolder:          "",
		ErrorMessageTimeout:       60,
		ErrorMessageRetries:       10,
		SetBlockProfileRate:       0,
		EnableSocket:              true,
		EnableSignatureCheck:      false,
		EnableAclCheck:            false,
		LogLevel:                  "debug",
		LogConsoleTimestamps:      false,
		KeepPublishersAliveFor:    10,
		JetstreamsConsume:         "",

		StartPubHello:           30,
		EnableKeyUpdates:        false,
		EnableAclUpdates:        false,
		IsCentralErrorLogger:    false,
		StartSubHello:           true,
		StartSubFileAppend:      true,
		StartSubFile:            true,
		StartSubCopySrc:         true,
		StartSubCopyDst:         true,
		StartSubCliCommand:      true,
		StartSubConsole:         true,
		StartSubHttpGet:         true,
		StartSubTailFile:        true,
		StartSubCliCommandCont:  true,
		IsCentralAuth:           false,
		StartJetstreamPublisher: true,
		StartJetstreamConsumer:  true,
	}
	return c
}

func CheckEnv[T any](key string, v T) any {
	val, ok := os.LookupEnv(key)
	if !ok {
		return v
	}

	switch any(v).(type) {
	case int:
		n, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("error: failed to convert env to int: %v\n", n)
		}
		return n
	case string:
		return val
	case bool:
		switch {
		case val == "true" || val == "1":
			return true
		case val == "false" || val == "0" || val == "":
			return true
		}

	}

	return nil
}
