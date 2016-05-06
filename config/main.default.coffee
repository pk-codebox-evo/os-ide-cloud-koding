traverse              = require 'traverse'
log                   = console.log
fs                    = require 'fs'
os                    = require 'os'
path                  = require 'path'
{ isAllowed }         = require '../deployment/grouptoenvmapping'

Configuration = (options = {}) ->

  options.domains =
    base  : 'koding.com'
    mail  : 'koding.com'
    main  : 'dev.koding.com'
    port  : '8090'

  options.boot2dockerbox or= if os.type() is "Darwin" then "192.168.59.103" else "localhost"
  options.serviceHost = options.boot2dockerbox
  options.publicPort or= "8090"
  options.hostname or= "dev.koding.com"
  options.protocol or= "http:"
  options.publicHostname or= "#{options.protocol}//#{options.hostname}"
  options.region or= "default"
  options.configName or= "default"
  options.environment or= "default"
  options.projectRoot or= path.join __dirname, '/..'
  options.version or= "2.0" # TBD
  options.build or= "1111"
  options.tunnelUrl or= "http://devtunnelproxy.koding.com"
  options.userSitesDomain or= "dev.koding.io"
  options.defaultEmail or= "hello@#{options.domains.mail}"
  options.recaptchaEnabled or= no
  options.debugGithubAPI or= yes
  options.autoConfirmAccounts or= yes
  options.vmwatcherConnectToKlient = no
  options.secureCookie = no
  options.algoliaIndexSuffix = ".#{ os.hostname() }"
  options.socialQueueName = "koding-social-#{options.configName}"
  options.sendEventsToSegment = yes
  options.scheme = 'http'
  options.suppressLogs = no
  options.paymentBlockDuration = 2 * 60 * 1000 # 2 minutes
  options.host or= options.hostname
  options.credentialPath or= "#{options.projectRoot}/config/credentials.#{options.environment}.coffee"

  customDomain =
    public  : "#{options.scheme}://#{options.host}#{if options.publicPort is "80" then "" else ":" + options.publicPort}"
    public_ : options.host
    local   : "http://127.0.0.1#{if options.publicPort is "80" then "" else ":" + options.publicPort}"
    local_  : "127.0.0.1#{if options.publicPort is "80" then "" else ":" + options.publicPort}"
    port    : parseInt(options.publicPort, 10)

  options.customDomain = customDomain
  credentials = require("./credentials.#{options.environment}")(options)
  worker_ci_test = require './aws/worker_ci_test_key.json'

  # if you want to disable a feature add here with "true" value do not forget to
  # add corresponding go struct properties
  # "true" value is used because of Go's default value for boolean properties is
  # false, so all the features are enabled as default, you dont have to define
  # features everywhere
  options.disabledFeatures =
    moderation : yes
    teams      : no
    botchannel : yes

  KONFIG = require('./generateKonfig')(options, credentials)
  KONFIG.workers = require('./workers')(KONFIG, options, credentials)
  KONFIG.client.runtimeOptions = require('./generateRuntimeConfig')(KONFIG, credentials, options)

  generateSh = "#{options.projectRoot}/config/generate.sh"

  # BUG(rjeczalik): The Configuration gets executed twice, once with uninitialized
  # options, which makes the following code execute generate.sh with
  # options.projectRoot equal to "/opt/koding/config". The todo here is to
  # fix it so it gets executed only once one remove the workaround.
  if fs.existsSync generateSh
    { execFile } = require 'child_process'

    execFile generateSh, ["#{KONFIG.kontrol.url}"], (err, stdout, stderr) ->
      process.stderr.write stdout
      process.stderr.write stderr

      if err
        console.log """
          failed to run #{options.projectRoot}/config/generate.sh (error: #{err})
          please execute it manually and most likely install missing dependencies
        """
        process.exit 1

  options.disabledWorkers = [
    "algoliaconnector"
    "paymentwebhook"
  # "gatekeeper"
    "vmwatcher"
  ]

  KONFIG.supervisord =
    logdir   : "#{options.projectRoot}/.logs"
    rundir   : "#{options.projectRoot}/.supervisor"
    minfds   : 1024
    minprocs : 200

  KONFIG.supervisord.output_path = "#{options.projectRoot}/supervisord.conf"

  KONFIG.supervisord.unix_http_server =
    file : "#{KONFIG.supervisord.rundir}/supervisor.sock"

  KONFIG.JSON = JSON.stringify KONFIG
  KONFIG.ENV = (require "../deployment/envvar.coffee").create KONFIG
  KONFIG.supervisorConf = (require "../deployment/supervisord.coffee").create KONFIG
  KONFIG.nginxConf = (require "../deployment/nginx.coffee").create KONFIG, options.environment
  KONFIG.runFile = require('./generateRunFile').dev(KONFIG, options, credentials)
  KONFIG.configCheckExempt = []

  return KONFIG

module.exports = Configuration
