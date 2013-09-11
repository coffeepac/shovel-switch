zi-relay
========

Daemon that turns a list of services on/off
depending on what link ZI is routing through.

ToDo:
=====
- add 'scheduled quit' instead of forcing user to hope that no external
  commands are running
- correct pidfile handling (leaves it behind during hard quit)
- quit chan messages get sent to all funcs for a clean exit
- make managed services config driven, not hardcoded
- consider having one generalized function, not one for each type
