kd = require 'kd'
JView = require 'app/jview'
Events = require '../events'
globals = require 'globals'


module.exports = class CredentialListItem extends kd.ListItemView

  JView.mixin @prototype

  constructor: (options = {}, data) ->

    options.cssClass = kd.utils.curry 'credential', options.cssClass

    super options, data

    { provider } = @getData()
    providerColor = globals.config.providers[provider]?.color ? '#666666'

    handle = (action) => =>
      @getDelegate().emit 'ItemAction', { action, item: this }

    @checkBox = new kd.CustomCheckBox
      defaultValue : off

    @preview = new kd.ButtonView
      cssClass: 'show'
      callback: handle 'ShowItem'

    @delete = new kd.ButtonView
      cssClass: 'delete'
      callback: handle 'RemoveItem'

    @edit = new kd.ButtonView
      cssClass: 'edit'
      callback: handle 'EditItem'

    @provider    = new kd.CustomHTMLView
      cssClass   : 'provider'
      partial    : provider
      attributes :
        style    : "background-color: #{providerColor}"
      click      : (event) =>
        @getDelegate().emit Events.CredentialFilterChanged, provider
        kd.utils.stopDOMEvent event


    @on 'click', (event) =>
      unless 'checkbox' in event.target.classList
        @select not @isSelected(), userAction = yes
        kd.utils.stopDOMEvent event



  select: (state = yes, userAction = no) ->
    @checkBox.setValue state
    if userAction
      @getDelegate().emit Events.CredentialSelectionChanged, this, state


  isSelected: ->
    @checkBox.getValue()


  pistachio: ->

    '''
    {{> @checkBox}} {span.title{#(title)}}
    {{> @edit}} {{> @delete}} {{> @preview}}
    {{> @provider}}
    '''
