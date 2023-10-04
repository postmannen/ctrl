package ctrl

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
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
	// REQKeysRequestUpdateInterval in seconds
	REQKeysRequestUpdateInterval int `comment:"REQKeysRequestUpdateInterval in seconds"`
	// REQAclRequestUpdateInterval in seconds
	REQAclRequestUpdateInterval int `comment:"REQAclRequestUpdateInterval in seconds"`
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
	// The host and port to expose the data folder, <host>:<port>
	ExposeDataFolder string `comment:"The host and port to expose the data folder, <host>:<port>"`
	// Timeout in seconds for error messages
	ErrorMessageTimeout int `comment:"Timeout in seconds for error messages"`
	// Retries for error messages
	ErrorMessageRetries int `comment:"Retries for error messages"`
	// Compression z for zstd or g for gzip
	Compression string `comment:"Compression z for zstd or g for gzip"`
	// Serialization, supports cbor or gob,default is gob. Enable cbor by setting the string value cbor
	Serialization string `comment:"Serialization, supports cbor or gob,default is gob. Enable cbor by setting the string value cbor"`
	// SetBlockProfileRate for block profiling
	SetBlockProfileRate int `comment:"SetBlockProfileRate for block profiling"`
	// EnableSocket for enabling the creation of a ctrl.sock file
	EnableSocket bool `comment:"EnableSocket for enabling the creation of a ctrl.sock file"`
	// EnableSignatureCheck to enable signature checking
	EnableSignatureCheck bool `comment:"EnableSignatureCheck to enable signature checking"`
	// EnableAclCheck to enable ACL checking
	EnableAclCheck bool `comment:"EnableAclCheck to enable ACL checking"`
	// IsCentralAuth, enable to make this instance take the role as the central auth server
	IsCentralAuth bool `comment:"IsCentralAuth, enable to make this instance take the role as the central auth server"`
	// EnableDebug will also enable printing all the messages received in the errorKernel to STDERR.
	EnableDebug bool `comment:"EnableDebug will also enable printing all the messages received in the errorKernel to STDERR."`
	// LogLevel
	LogLevel             string `comment:"LogLevel error/info/warning/debug/none."`
	LogConsoleTimestamps bool   `comment:"LogConsoleTimestamps true/false for enabling or disabling timestamps when printing errors and information to stderr"`
	// KeepPublishersAliveFor number of seconds
	// Timer that will be used for when to remove the sub process
	// publisher. The timer is reset each time a message is published with
	// the process, so the sub process publisher will not be removed until
	// it have not received any messages for the given amount of time.
	KeepPublishersAliveFor int `comment:"KeepPublishersAliveFor number of seconds Timer that will be used for when to remove the sub process publisher. The timer is reset each time a message is published with the process, so the sub process publisher will not be removed until it have not received any messages for the given amount of time."`

	// StartPubREQHello, sets the interval in seconds for how often we send hello messages to central server
	StartPubREQHello int `comment:"StartPubREQHello, sets the interval in seconds for how often we send hello messages to central server"`
	// Enable the updates of public keys
	EnableKeyUpdates bool `comment:"Enable the updates of public keys"`

	// Enable the updates of acl's
	EnableAclUpdates bool `comment:"Enable the updates of acl's"`

	// Start the central error logger.
	IsCentralErrorLogger bool `comment:"Start the central error logger."`
	// Start subscriber for hello messages
	StartSubREQHello bool `comment:"Start subscriber for hello messages"`
	// Start subscriber for text logging
	StartSubREQToFileAppend bool `comment:"Start subscriber for text logging"`
	// Start subscriber for writing to file
	StartSubREQToFile bool `comment:"Start subscriber for writing to file"`
	// Start subscriber for writing to file without ACK
	StartSubREQToFileNACK bool `comment:"Start subscriber for writing to file without ACK"`
	// Start subscriber for reading files to copy
	StartSubREQCopySrc bool `comment:"Start subscriber for reading files to copy"`
	// Start subscriber for writing copied files to disk
	StartSubREQCopyDst bool `comment:"Start subscriber for writing copied files to disk"`
	// Start subscriber for Echo Request
	StartSubREQCliCommand bool `comment:"Start subscriber for CLICommandRequest"`
	// Start subscriber for REQToConsole
	StartSubREQToConsole bool `comment:"Start subscriber for REQToConsole"`
	// Start subscriber for REQHttpGet
	StartSubREQHttpGet bool `comment:"Start subscriber for REQHttpGet"`
	// Start subscriber for REQHttpGetScheduled
	StartSubREQHttpGetScheduled bool `comment:"Start subscriber for REQHttpGetScheduled"`
	// Start subscriber for tailing log files
	StartSubREQTailFile bool `comment:"Start subscriber for tailing log files"`
	// Start subscriber for continously delivery of output from cli commands.
	StartSubREQCliCommandCont bool `comment:"Start subscriber for continously delivery of output from cli commands."`
}

// ConfigurationFromFile should have the same structure as
// Configuration. This structure is used when parsing the
// configuration values from file, so we are able to detect
// if a value were given or not when parsing.
type ConfigurationFromFile struct {
	ConfigFolder                 *string
	RingBufferPersistStore       *bool
	RingBufferSize               *int
	SocketFolder                 *string
	ReadFolder                   *string
	EnableReadFolder             *bool
	TCPListener                  *string
	HTTPListener                 *string
	DatabaseFolder               *string
	NodeName                     *string
	BrokerAddress                *string
	NatsConnOptTimeout           *int
	NatsConnectRetryInterval     *int
	NatsReconnectJitter          *int
	NatsReconnectJitterTLS       *int
	REQKeysRequestUpdateInterval *int
	REQAclRequestUpdateInterval  *int
	ProfilingPort                *string
	PromHostAndPort              *string
	DefaultMessageTimeout        *int
	DefaultMessageRetries        *int
	DefaultMethodTimeout         *int
	SubscribersDataFolder        *string
	CentralNodeName              *string
	RootCAPath                   *string
	NkeySeedFile                 *string
	NkeyFromED25519SSHKeyFile    *string
	ExposeDataFolder             *string
	ErrorMessageTimeout          *int
	ErrorMessageRetries          *int
	Compression                  *string
	Serialization                *string
	SetBlockProfileRate          *int
	EnableSocket                 *bool
	EnableSignatureCheck         *bool
	EnableAclCheck               *bool
	IsCentralAuth                *bool
	EnableDebug                  *bool
	LogLevel                     *string
	LogConsoleTimestamps         *bool
	KeepPublishersAliveFor       *int

	StartPubREQHello            *int
	EnableKeyUpdates            *bool
	EnableAclUpdates            *bool
	IsCentralErrorLogger        *bool
	StartSubREQHello            *bool
	StartSubREQToFileAppend     *bool
	StartSubREQToFile           *bool
	StartSubREQToFileNACK       *bool
	StartSubREQCopySrc          *bool
	StartSubREQCopyDst          *bool
	StartSubREQCliCommand       *bool
	StartSubREQToConsole        *bool
	StartSubREQHttpGet          *bool
	StartSubREQHttpGetScheduled *bool
	StartSubREQTailFile         *bool
	StartSubREQCliCommandCont   *bool
}

// NewConfiguration will return a *Configuration.
func NewConfiguration() *Configuration {
	c := Configuration{}
	return &c
}

// Get a Configuration struct with the default values set.
func newConfigurationDefaults() Configuration {
	c := Configuration{
		ConfigFolder:                 "./etc/",
		SocketFolder:                 "./tmp",
		ReadFolder:                   "./readfolder",
		EnableReadFolder:             true,
		TCPListener:                  "",
		HTTPListener:                 "",
		DatabaseFolder:               "./var/lib",
		NodeName:                     "",
		BrokerAddress:                "127.0.0.1:4222",
		NatsConnOptTimeout:           20,
		NatsConnectRetryInterval:     10,
		NatsReconnectJitter:          100,
		NatsReconnectJitterTLS:       1,
		REQKeysRequestUpdateInterval: 60,
		REQAclRequestUpdateInterval:  60,
		ProfilingPort:                "",
		PromHostAndPort:              "",
		DefaultMessageTimeout:        10,
		DefaultMessageRetries:        1,
		DefaultMethodTimeout:         10,
		SubscribersDataFolder:        "./data",
		CentralNodeName:              "central",
		RootCAPath:                   "",
		NkeySeedFile:                 "",
		NkeyFromED25519SSHKeyFile:    "",
		ExposeDataFolder:             "",
		ErrorMessageTimeout:          60,
		ErrorMessageRetries:          10,
		Compression:                  "z",
		Serialization:                "cbor",
		SetBlockProfileRate:          0,
		EnableSocket:                 true,
		EnableSignatureCheck:         false,
		EnableAclCheck:               false,
		IsCentralAuth:                false,
		EnableDebug:                  false,
		LogLevel:                     "debug",
		LogConsoleTimestamps:         false,
		KeepPublishersAliveFor:       10,

		StartPubREQHello:            30,
		EnableKeyUpdates:            false,
		EnableAclUpdates:            false,
		IsCentralErrorLogger:        false,
		StartSubREQHello:            true,
		StartSubREQToFileAppend:     true,
		StartSubREQToFile:           true,
		StartSubREQToFileNACK:       true,
		StartSubREQCopySrc:          true,
		StartSubREQCopyDst:          true,
		StartSubREQCliCommand:       true,
		StartSubREQToConsole:        true,
		StartSubREQHttpGet:          true,
		StartSubREQHttpGetScheduled: true,
		StartSubREQTailFile:         true,
		StartSubREQCliCommandCont:   true,
	}
	return c
}

// Check if all values are present in config file, and if not
// found use the default value.
func checkConfigValues(cf ConfigurationFromFile) Configuration {
	var conf Configuration
	cd := newConfigurationDefaults()

	if cf.ConfigFolder == nil {
		conf.ConfigFolder = cd.ConfigFolder
	} else {
		conf.ConfigFolder = *cf.ConfigFolder
	}
	if cf.SocketFolder == nil {
		conf.SocketFolder = cd.SocketFolder
	} else {
		conf.SocketFolder = *cf.SocketFolder
	}
	if cf.ReadFolder == nil {
		conf.ReadFolder = cd.ReadFolder
	} else {
		conf.ReadFolder = *cf.ReadFolder
	}
	if cf.EnableReadFolder == nil {
		conf.EnableReadFolder = cd.EnableReadFolder
	} else {
		conf.EnableReadFolder = *cf.EnableReadFolder
	}
	if cf.TCPListener == nil {
		conf.TCPListener = cd.TCPListener
	} else {
		conf.TCPListener = *cf.TCPListener
	}
	if cf.HTTPListener == nil {
		conf.HTTPListener = cd.HTTPListener
	} else {
		conf.HTTPListener = *cf.HTTPListener
	}
	if cf.DatabaseFolder == nil {
		conf.DatabaseFolder = cd.DatabaseFolder
	} else {
		conf.DatabaseFolder = *cf.DatabaseFolder
	}
	if cf.NodeName == nil {
		conf.NodeName = cd.NodeName
	} else {
		conf.NodeName = *cf.NodeName
	}
	if cf.BrokerAddress == nil {
		conf.BrokerAddress = cd.BrokerAddress
	} else {
		conf.BrokerAddress = *cf.BrokerAddress
	}
	if cf.NatsConnOptTimeout == nil {
		conf.NatsConnOptTimeout = cd.NatsConnOptTimeout
	} else {
		conf.NatsConnOptTimeout = *cf.NatsConnOptTimeout
	}
	if cf.NatsConnectRetryInterval == nil {
		conf.NatsConnectRetryInterval = cd.NatsConnectRetryInterval
	} else {
		conf.NatsConnectRetryInterval = *cf.NatsConnectRetryInterval
	}
	if cf.NatsReconnectJitter == nil {
		conf.NatsReconnectJitter = cd.NatsReconnectJitter
	} else {
		conf.NatsReconnectJitter = *cf.NatsReconnectJitter
	}
	if cf.NatsReconnectJitterTLS == nil {
		conf.NatsReconnectJitterTLS = cd.NatsReconnectJitterTLS
	} else {
		conf.NatsReconnectJitterTLS = *cf.NatsReconnectJitterTLS
	}
	if cf.REQKeysRequestUpdateInterval == nil {
		conf.REQKeysRequestUpdateInterval = cd.REQKeysRequestUpdateInterval
	} else {
		conf.REQKeysRequestUpdateInterval = *cf.REQKeysRequestUpdateInterval
	}
	if cf.REQAclRequestUpdateInterval == nil {
		conf.REQAclRequestUpdateInterval = cd.REQAclRequestUpdateInterval
	} else {
		conf.REQAclRequestUpdateInterval = *cf.REQAclRequestUpdateInterval
	}
	if cf.ProfilingPort == nil {
		conf.ProfilingPort = cd.ProfilingPort
	} else {
		conf.ProfilingPort = *cf.ProfilingPort
	}
	if cf.PromHostAndPort == nil {
		conf.PromHostAndPort = cd.PromHostAndPort
	} else {
		conf.PromHostAndPort = *cf.PromHostAndPort
	}
	if cf.DefaultMessageTimeout == nil {
		conf.DefaultMessageTimeout = cd.DefaultMessageTimeout
	} else {
		conf.DefaultMessageTimeout = *cf.DefaultMessageTimeout
	}
	if cf.DefaultMessageRetries == nil {
		conf.DefaultMessageRetries = cd.DefaultMessageRetries
	} else {
		conf.DefaultMessageRetries = *cf.DefaultMessageRetries
	}
	if cf.DefaultMethodTimeout == nil {
		conf.DefaultMethodTimeout = cd.DefaultMethodTimeout
	} else {
		conf.DefaultMethodTimeout = *cf.DefaultMethodTimeout
	}
	if cf.SubscribersDataFolder == nil {
		conf.SubscribersDataFolder = cd.SubscribersDataFolder
	} else {
		conf.SubscribersDataFolder = *cf.SubscribersDataFolder
	}
	if cf.CentralNodeName == nil {
		conf.CentralNodeName = cd.CentralNodeName
	} else {
		conf.CentralNodeName = *cf.CentralNodeName
	}
	if cf.RootCAPath == nil {
		conf.RootCAPath = cd.RootCAPath
	} else {
		conf.RootCAPath = *cf.RootCAPath
	}
	if cf.NkeySeedFile == nil {
		conf.NkeySeedFile = cd.NkeySeedFile
	} else {
		conf.NkeySeedFile = *cf.NkeySeedFile
	}
	if cf.NkeyFromED25519SSHKeyFile == nil {
		conf.NkeyFromED25519SSHKeyFile = cd.NkeyFromED25519SSHKeyFile
	} else {
		conf.NkeyFromED25519SSHKeyFile = *cf.NkeyFromED25519SSHKeyFile
	}
	if cf.ExposeDataFolder == nil {
		conf.ExposeDataFolder = cd.ExposeDataFolder
	} else {
		conf.ExposeDataFolder = *cf.ExposeDataFolder
	}
	if cf.ErrorMessageTimeout == nil {
		conf.ErrorMessageTimeout = cd.ErrorMessageTimeout
	} else {
		conf.ErrorMessageTimeout = *cf.ErrorMessageTimeout
	}
	if cf.ErrorMessageRetries == nil {
		conf.ErrorMessageRetries = cd.ErrorMessageRetries
	} else {
		conf.ErrorMessageRetries = *cf.ErrorMessageRetries
	}
	if cf.Compression == nil {
		conf.Compression = cd.Compression
	} else {
		conf.Compression = *cf.Compression
	}
	if cf.Serialization == nil {
		conf.Serialization = cd.Serialization
	} else {
		conf.Serialization = *cf.Serialization
	}
	if cf.SetBlockProfileRate == nil {
		conf.SetBlockProfileRate = cd.SetBlockProfileRate
	} else {
		conf.SetBlockProfileRate = *cf.SetBlockProfileRate
	}
	if cf.EnableSocket == nil {
		conf.EnableSocket = cd.EnableSocket
	} else {
		conf.EnableSocket = *cf.EnableSocket
	}
	if cf.EnableSignatureCheck == nil {
		conf.EnableSignatureCheck = cd.EnableSignatureCheck
	} else {
		conf.EnableSignatureCheck = *cf.EnableSignatureCheck
	}
	if cf.EnableAclCheck == nil {
		conf.EnableAclCheck = cd.EnableAclCheck
	} else {
		conf.EnableAclCheck = *cf.EnableAclCheck
	}
	if cf.IsCentralAuth == nil {
		conf.IsCentralAuth = cd.IsCentralAuth
	} else {
		conf.IsCentralAuth = *cf.IsCentralAuth
	}
	if cf.EnableDebug == nil {
		conf.EnableDebug = cd.EnableDebug
	} else {
		conf.EnableDebug = *cf.EnableDebug
	}
	if cf.LogLevel == nil {
		conf.LogLevel = cd.LogLevel
	} else {
		conf.LogLevel = *cf.LogLevel
	}
	if cf.LogConsoleTimestamps == nil {
		conf.LogConsoleTimestamps = cd.LogConsoleTimestamps
	} else {
		conf.LogConsoleTimestamps = *cf.LogConsoleTimestamps
	}
	if cf.KeepPublishersAliveFor == nil {
		conf.KeepPublishersAliveFor = cd.KeepPublishersAliveFor
	} else {
		conf.KeepPublishersAliveFor = *cf.KeepPublishersAliveFor
	}

	// --- Start pub/sub

	if cf.StartPubREQHello == nil {
		conf.StartPubREQHello = cd.StartPubREQHello
	} else {
		conf.StartPubREQHello = *cf.StartPubREQHello
	}
	if cf.EnableKeyUpdates == nil {
		conf.EnableKeyUpdates = cd.EnableKeyUpdates
	} else {
		conf.EnableKeyUpdates = *cf.EnableKeyUpdates
	}

	if cf.EnableAclUpdates == nil {
		conf.EnableAclUpdates = cd.EnableAclUpdates
	} else {
		conf.EnableAclUpdates = *cf.EnableAclUpdates
	}

	if cf.IsCentralErrorLogger == nil {
		conf.IsCentralErrorLogger = cd.IsCentralErrorLogger
	} else {
		conf.IsCentralErrorLogger = *cf.IsCentralErrorLogger
	}
	if cf.StartSubREQHello == nil {
		conf.StartSubREQHello = cd.StartSubREQHello
	} else {
		conf.StartSubREQHello = *cf.StartSubREQHello
	}
	if cf.StartSubREQToFileAppend == nil {
		conf.StartSubREQToFileAppend = cd.StartSubREQToFileAppend
	} else {
		conf.StartSubREQToFileAppend = *cf.StartSubREQToFileAppend
	}
	if cf.StartSubREQToFile == nil {
		conf.StartSubREQToFile = cd.StartSubREQToFile
	} else {
		conf.StartSubREQToFile = *cf.StartSubREQToFile
	}
	if cf.StartSubREQToFileNACK == nil {
		conf.StartSubREQToFileNACK = cd.StartSubREQToFileNACK
	} else {
		conf.StartSubREQToFileNACK = *cf.StartSubREQToFileNACK
	}
	if cf.StartSubREQCopySrc == nil {
		conf.StartSubREQCopySrc = cd.StartSubREQCopySrc
	} else {
		conf.StartSubREQCopySrc = *cf.StartSubREQCopySrc
	}
	if cf.StartSubREQCopyDst == nil {
		conf.StartSubREQCopyDst = cd.StartSubREQCopyDst
	} else {
		conf.StartSubREQCopyDst = *cf.StartSubREQCopyDst
	}
	if cf.StartSubREQCliCommand == nil {
		conf.StartSubREQCliCommand = cd.StartSubREQCliCommand
	} else {
		conf.StartSubREQCliCommand = *cf.StartSubREQCliCommand
	}
	if cf.StartSubREQToConsole == nil {
		conf.StartSubREQToConsole = cd.StartSubREQToConsole
	} else {
		conf.StartSubREQToConsole = *cf.StartSubREQToConsole
	}
	if cf.StartSubREQHttpGet == nil {
		conf.StartSubREQHttpGet = cd.StartSubREQHttpGet
	} else {
		conf.StartSubREQHttpGet = *cf.StartSubREQHttpGet
	}
	if cf.StartSubREQHttpGetScheduled == nil {
		conf.StartSubREQHttpGetScheduled = cd.StartSubREQHttpGetScheduled
	} else {
		conf.StartSubREQHttpGetScheduled = *cf.StartSubREQHttpGetScheduled
	}
	if cf.StartSubREQTailFile == nil {
		conf.StartSubREQTailFile = cd.StartSubREQTailFile
	} else {
		conf.StartSubREQTailFile = *cf.StartSubREQTailFile
	}
	if cf.StartSubREQCliCommandCont == nil {
		conf.StartSubREQCliCommandCont = cd.StartSubREQCliCommandCont
	} else {
		conf.StartSubREQCliCommandCont = *cf.StartSubREQCliCommandCont
	}

	return conf
}

// CheckFlags will parse all flags
func (c *Configuration) CheckFlags(version string) error {

	// Create an empty default config
	var fc Configuration

	// Set default configfolder if no env was provided.
	configFolder := os.Getenv("CONFIG_FOLDER")

	if configFolder == "" {
		configFolder = "./etc/"
	}

	// Read file config. Set system default if it can't find config file.
	fc, err := c.ReadConfigFile(configFolder)
	if err != nil {
		log.Printf("%v\n", err)
		fc = newConfigurationDefaults()
	}

	if configFolder == "" {
		fc.ConfigFolder = "./etc/"
	} else {
		fc.ConfigFolder = configFolder
	}

	*c = fc

	//flag.StringVar(&c.ConfigFolder, "configFolder", fc.ConfigFolder, "Defaults to ./usr/local/ctrl/etc/. *NB* This flag is not used, if your config file are located somwhere else than default set the location in an env variable named CONFIGFOLDER")
	flag.StringVar(&c.SocketFolder, "socketFolder", fc.SocketFolder, "folder who contains the socket file. Defaults to ./tmp/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.ReadFolder, "readFolder", fc.ReadFolder, "folder who contains the readfolder. Defaults to ./readfolder/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.TCPListener, "tcpListener", fc.TCPListener, "start up a TCP listener in addition to the Unix Socket, to give messages to the system. e.g. localhost:8888. No value means not to start the listener, which is default. NB: You probably don't want to start this on any other interface than localhost")
	flag.StringVar(&c.HTTPListener, "httpListener", fc.HTTPListener, "start up a HTTP listener in addition to the Unix Socket, to give messages to the system. e.g. localhost:8888. No value means not to start the listener, which is default. NB: You probably don't want to start this on any other interface than localhost")
	flag.StringVar(&c.DatabaseFolder, "databaseFolder", fc.DatabaseFolder, "folder who contains the database file. Defaults to ./var/lib/. If other folder is used this flag must be specified at startup.")
	flag.StringVar(&c.NodeName, "nodeName", fc.NodeName, "some unique string to identify this Edge unit")
	flag.StringVar(&c.BrokerAddress, "brokerAddress", fc.BrokerAddress, "the address of the message broker")
	flag.IntVar(&c.NatsConnOptTimeout, "natsConnOptTimeout", fc.NatsConnOptTimeout, "default nats client conn timeout in seconds")
	flag.IntVar(&c.NatsConnectRetryInterval, "natsConnectRetryInterval", fc.NatsConnectRetryInterval, "default nats retry connect interval in seconds.")
	flag.IntVar(&c.NatsReconnectJitter, "natsReconnectJitter", fc.NatsReconnectJitter, "default nats ReconnectJitter interval in milliseconds.")
	flag.IntVar(&c.NatsReconnectJitterTLS, "natsReconnectJitterTLS", fc.NatsReconnectJitterTLS, "default nats ReconnectJitterTLS interval in seconds.")
	flag.IntVar(&c.REQKeysRequestUpdateInterval, "REQKeysRequestUpdateInterval", fc.REQKeysRequestUpdateInterval, "default interval in seconds for asking the central for public keys")
	flag.IntVar(&c.REQAclRequestUpdateInterval, "REQAclRequestUpdateInterval", fc.REQAclRequestUpdateInterval, "default interval in seconds for asking the central for acl updates")
	flag.StringVar(&c.ProfilingPort, "profilingPort", fc.ProfilingPort, "The number of the profiling port")
	flag.StringVar(&c.PromHostAndPort, "promHostAndPort", fc.PromHostAndPort, "host and port for prometheus listener, e.g. localhost:2112")
	flag.IntVar(&c.DefaultMessageTimeout, "defaultMessageTimeout", fc.DefaultMessageTimeout, "default message timeout in seconds. This can be overridden on the message level")
	flag.IntVar(&c.DefaultMessageRetries, "defaultMessageRetries", fc.DefaultMessageRetries, "default amount of retries that will be done before a message is thrown away, and out of the system")
	flag.IntVar(&c.DefaultMethodTimeout, "defaultMethodTimeout", fc.DefaultMethodTimeout, "default amount of seconds a request method max will be allowed to run")
	flag.StringVar(&c.SubscribersDataFolder, "subscribersDataFolder", fc.SubscribersDataFolder, "The data folder where subscribers are allowed to write their data if needed")
	flag.StringVar(&c.CentralNodeName, "centralNodeName", fc.CentralNodeName, "The name of the central node to receive messages published by this node")
	flag.StringVar(&c.RootCAPath, "rootCAPath", fc.RootCAPath, "If TLS, enter the path for where to find the root CA certificate")
	flag.StringVar(&c.NkeyFromED25519SSHKeyFile, "nkeyFromED25519SSHKeyFile", fc.NkeyFromED25519SSHKeyFile, "The full path of the nkeys seed file")
	flag.StringVar(&c.NkeySeedFile, "nkeySeedFile", fc.NkeySeedFile, "Full path to the ED25519 SSH private key. Will generate the NKEY Seed from an SSH ED25519 private key file. NB: This option will take precedence over NkeySeedFile if specified")
	flag.StringVar(&c.ExposeDataFolder, "exposeDataFolder", fc.ExposeDataFolder, "If set the data folder will be exposed on the given host:port. Default value is not exposed at all")
	flag.IntVar(&c.ErrorMessageTimeout, "errorMessageTimeout", fc.ErrorMessageTimeout, "The number of seconds to wait for an error message to time out")
	flag.IntVar(&c.ErrorMessageRetries, "errorMessageRetries", fc.ErrorMessageRetries, "The number of if times to retry an error message before we drop it")
	flag.StringVar(&c.Compression, "compression", fc.Compression, "compression method to use. defaults to no compression, z = zstd, g = gzip. Undefined value will default to no compression")
	flag.StringVar(&c.Serialization, "serialization", fc.Serialization, "Serialization method to use. defaults to gob, other values are = cbor. Undefined value will default to gob")
	flag.IntVar(&c.SetBlockProfileRate, "setBlockProfileRate", fc.SetBlockProfileRate, "Enable block profiling by setting the value to f.ex. 1. 0 = disabled")
	flag.BoolVar(&c.EnableSocket, "enableSocket", fc.EnableSocket, "true/false, for enabling the creation of ctrl.sock file")
	flag.BoolVar(&c.EnableSignatureCheck, "enableSignatureCheck", fc.EnableSignatureCheck, "true/false *TESTING* enable signature checking.")
	flag.BoolVar(&c.EnableAclCheck, "enableAclCheck", fc.EnableAclCheck, "true/false *TESTING* enable Acl checking.")
	flag.BoolVar(&c.IsCentralAuth, "isCentralAuth", fc.IsCentralAuth, "true/false, *TESTING* is this the central auth server")
	flag.BoolVar(&c.EnableDebug, "enableDebug", fc.EnableDebug, "true/false, will enable debug logging so all messages sent to the errorKernel will also be printed to STDERR")
	flag.StringVar(&c.LogLevel, "logLevel", fc.LogLevel, "error/info/warning/debug/none")
	flag.BoolVar(&c.LogConsoleTimestamps, "LogConsoleTimestamps", fc.LogConsoleTimestamps, "true/false for enabling or disabling timestamps when printing errors and information to stderr")
	flag.IntVar(&c.KeepPublishersAliveFor, "keepPublishersAliveFor", fc.KeepPublishersAliveFor, "The amount of time we allow a publisher to stay alive without receiving any messages to publish")

	// Start of Request publishers/subscribers

	flag.IntVar(&c.StartPubREQHello, "startPubREQHello", fc.StartPubREQHello, "Make the current node send hello messages to central at given interval in seconds")

	flag.BoolVar(&c.EnableKeyUpdates, "EnableKeyUpdates", fc.EnableKeyUpdates, "true/false")

	flag.BoolVar(&c.EnableAclUpdates, "EnableAclUpdates", fc.EnableAclUpdates, "true/false")

	flag.BoolVar(&c.IsCentralErrorLogger, "isCentralErrorLogger", fc.IsCentralErrorLogger, "true/false")
	flag.BoolVar(&c.StartSubREQHello, "startSubREQHello", fc.StartSubREQHello, "true/false")
	flag.BoolVar(&c.StartSubREQToFileAppend, "startSubREQToFileAppend", fc.StartSubREQToFileAppend, "true/false")
	flag.BoolVar(&c.StartSubREQToFile, "startSubREQToFile", fc.StartSubREQToFile, "true/false")
	flag.BoolVar(&c.StartSubREQToFileNACK, "startSubREQToFileNACK", fc.StartSubREQToFileNACK, "true/false")
	flag.BoolVar(&c.StartSubREQCopySrc, "startSubREQCopySrc", fc.StartSubREQCopySrc, "true/false")
	flag.BoolVar(&c.StartSubREQCopyDst, "startSubREQCopyDst", fc.StartSubREQCopyDst, "true/false")
	flag.BoolVar(&c.StartSubREQCliCommand, "startSubREQCliCommand", fc.StartSubREQCliCommand, "true/false")
	flag.BoolVar(&c.StartSubREQToConsole, "startSubREQToConsole", fc.StartSubREQToConsole, "true/false")
	flag.BoolVar(&c.StartSubREQHttpGet, "startSubREQHttpGet", fc.StartSubREQHttpGet, "true/false")
	flag.BoolVar(&c.StartSubREQHttpGetScheduled, "startSubREQHttpGetScheduled", fc.StartSubREQHttpGetScheduled, "true/false")
	flag.BoolVar(&c.StartSubREQTailFile, "startSubREQTailFile", fc.StartSubREQTailFile, "true/false")
	flag.BoolVar(&c.StartSubREQCliCommandCont, "startSubREQCliCommandCont", fc.StartSubREQCliCommandCont, "true/false")

	purgeBufferDB := flag.Bool("purgeBufferDB", false, "true/false, purge the incoming buffer db and all it's state")

	ver := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *ver {
		fmt.Printf("version %v\n", version)
		os.Exit(0)
	}

	// Check that mandatory flag values have been set.
	switch {
	case c.NodeName == "":
		return fmt.Errorf("error: the nodeName config option or flag cannot be empty, check -help")
	case c.CentralNodeName == "":
		return fmt.Errorf("error: the centralNodeName config option or flag cannot be empty, check -help")
	}

	if err := c.WriteConfigFile(); err != nil {
		log.Printf("error: checkFlags: failed writing config file: %v\n", err)
		os.Exit(1)
	}

	if *purgeBufferDB {
		fp := filepath.Join(c.DatabaseFolder, "incomingBuffer.db")
		err := os.Remove(fp)
		if err != nil {
			log.Printf("error: failed to purge buffer state database: %v\n", err)
		}

	}

	return nil
}

// Reads the current config file from disk.
func (c *Configuration) ReadConfigFile(configFolder string) (Configuration, error) {
	fPath := filepath.Join(configFolder, "config.toml")

	if _, err := os.Stat(fPath); os.IsNotExist(err) {
		return Configuration{}, fmt.Errorf("error: no config file found %v: %v", fPath, err)
	}

	f, err := os.OpenFile(fPath, os.O_RDONLY, 0660)
	if err != nil {
		return Configuration{}, fmt.Errorf("error: ReadConfigFile: failed to open file: %v", err)
	}
	defer f.Close()

	var cFile ConfigurationFromFile
	dec := toml.NewDecoder(f)
	err = dec.Decode(&cFile)
	if err != nil {
		log.Printf("error: decoding config.toml file. The program will automatically try to correct the problem, and use sane default where it kind find a value to use, but beware of this error in case the program start to behave in not expected ways: path=%v: err=%v", fPath, err)
	}

	// Check that all values read are ok.
	conf := checkConfigValues(cFile)

	return conf, nil
}

// WriteConfigFile will write the current config to file. If the file or the
// directory for the config file does not exist it will be created.
func (c *Configuration) WriteConfigFile() error {
	if _, err := os.Stat(c.ConfigFolder); os.IsNotExist(err) {
		err := os.MkdirAll(c.ConfigFolder, 0770)
		if err != nil {
			return fmt.Errorf("error: failed to create config directory %v: %v", c.ConfigFolder, err)
		}
	}

	fp := filepath.Join(c.ConfigFolder, "config.toml")

	f, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return fmt.Errorf("error: WriteConfigFile: failed to open file: %v", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Encode(c)

	return nil
}
