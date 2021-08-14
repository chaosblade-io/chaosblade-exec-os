#!/usr/bin/python
# @Author zhangfenghao

import os,sys
import getopt
import subprocess

def usage():
    print "Usage: %s -d disk -t [read/write/rw] -a [hang/throttle/recover/show] [-i iops -b bandwidth]" % sys.argv[0]
    print "       -d:    Disk to take action on, for example: sda/sdb/vda/vdb, see command: lsblk "
    print "       -t:    Io type to take effect, for example: read, write, read&write"
    print "       -a:    Action to take on device"
    print "                 'hang', block all IO's on a device"
    print "                 'throttle', set throttled iops/bps on a device"
    print "                 'recover', recover io setting without hang or throttle"
    print "                 'show', show current config settings"
    print "       -i:    Set iops for a device, only work for 'throttle'"
    print "       -b:    Set bandwidth for a device, only work for 'throttle', unit: bytes/s"

class DiskController(object):
    def __init__(self, disk, type, action, iops = None, bps = None):
        self._deviceName = disk
        self._deviceId = self._getDeviceId(disk)
        self._type = type
        self._action = action
        self._iops = iops
        self._bps = bps

    def _executeCmd(self, cmd):
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True)
        (output, tmp) = p.communicate()
        error = p.wait()
        if error != 0:
            print "ERROR! Command: %s\nOutput: %s\nError code: %d\n" % (cmd, output, error)
            sys.exit(error)
        return output

    def _getDeviceId(self, disk):
        cmd = "lsblk |grep -w %s |grep disk |head -1 | awk '{print $2}'" % disk
        return self._executeCmd(cmd).strip()

    def _getIo(self, op):
        iops = "Infinite"
        bps = "Infinite"
        cmd = "sudo cat /sys/fs/cgroup/blkio/blkio.throttle.%s_iops_device" % (op)
        info = self._executeCmd(cmd).strip()
        if len(info) != 0:
            deviceId,iops = info.split(" ")
        cmd = "sudo cat /sys/fs/cgroup/blkio/blkio.throttle.%s_bps_device" % (op)
        info = self._executeCmd(cmd).strip()
        if len(info) != 0:
            deviceId,bps = info.split(" ")
        return "%-15s %-15s %-15s %-15s" % (self._deviceName, op, iops, bps)

    def _setIo(self, op, iops, bps):
        if iops != None:
            cmd = "sudo echo '%s %d' > /sys/fs/cgroup/blkio/blkio.throttle.%s_iops_device" % (self._deviceId, int(iops), op)
            self._executeCmd(cmd)
        if bps != None:
            cmd = "sudo echo '%s %d' > /sys/fs/cgroup/blkio/blkio.throttle.%s_bps_device" % (self._deviceId, int(bps), op)
            self._executeCmd(cmd)

    def _recover(self):
        if self._type == 'read':
            self._setIo("read", 0, 0)
        elif self._type == 'write':
            self._setIo("write", 0, 0)
        elif self._type == 'rw':
            self._setIo("read", 0, 0)
            self._setIo("write", 0, 0)
        else:
            print "Error! Invalid io type: %s" % self._type
            usage()
            sys.exit(1)

    def _hang(self):
        if self._type == 'read':
            self._setIo("read", 1, 1)
        elif self._type == 'write':
            self._setIo("write", 1, 1)
        elif self._type == 'rw':
            self._setIo("read", 1, 1)
            self._setIo("write", 1, 1)
        else:
            print "Error! Invalid io type: %s" % self._type
            usage()
            sys.exit(1)

    def _throttle(self):
        if self._type == 'read':
            self._setIo("read", self._iops, self._bps)
        elif self._type == 'write':
            self._setIo("write", self._iops, self._bps)
        elif self._type == 'rw':
            self._setIo("read", self._iops, self._bps)
            self._setIo("write", self._iops, self._bps)
        else:
            print "Error! Invalid io type: %s" % self._type
            usage()
            sys.exit(1)

    def _show(self):
        print "%-15s %-15s %-15s %-15s" % ("Device", "Operation", "IOPS", "Bandwidth(Bytes)")
        print self._getIo("read")
        print self._getIo("write")

    def process(self):
        if self._action == 'hang':
            self._hang()
        elif self._action == 'throttle':
            self._throttle()
        elif self._action == 'recover':
            self._recover()
        elif self._action == 'show':
            self._show()
        else:
            print "Error! Invalid action type: %s" % self._action
            usage()
            sys.exit(1)

def getOpt():
    opts,args = getopt.getopt(sys.argv[1:], "d:t:a:i:b:h")
    if len(opts) == 1:
        usage()
        sys.exit(1)

    disk = ""
    type = ""
    action = ""
    iops = None
    bps = None
    for opt,value in opts:
        if opt in ('-h', '--help'):
            usage()
            sys.exit(1)
        elif opt in ('-d', '--disk'):
            disk = value
        elif opt in ('-t', '--type'):
            type = value
        elif opt in ('-a', '--action'):
            action = value
        elif opt in ('-i', '--iops'):
            iops = value
        elif opt in ('-b', '--bps'):
            bps = value
    if disk == "" or type == "" or action == "":
        print "Error! Lack of parameters."
        usage()
        sys.exit(1)
    return disk, type, action, iops, bps

def main():
    disk,type,action,iops,bps = getOpt()
    controller = DiskController(disk, type, action, iops, bps)
    controller.process()

if __name__ == "__main__":
    main()
