DRIVE1 /dev/{{ OS_DRIVE }}

HOSTNAME {{ HOSTNAME }}

IMAGE {{ IMAGE }}

## Nicht setzbar wird vom controller gesetzt.
DRIVE1
HOSTNAME CentOS-85-64-minimal

## Defaults setzen wir.
SWRAID 0 # Setzen wir


## Configuration
network:
  ipv4Only: false

partitions:
  variante1: #normal
  - mountpoint:
    filesystem:
    size:
  variante2: #lvm
  - volumeGroup:
    size:
  variante3: #btrfs
  - name:
    mount: # all??

  lvmDefinitions:
  - volumeGroup:
    name:
    mount:
    filesystem:
    size:
  btrfsSubvolume:
  - subvolume:
    name:
    mount:







```shell
## ======================================================
##  Hetzner Online GmbH - installimage - standard config
## ======================================================


## ====================
##  HARD DISK DRIVE(S):
## ====================

## PLEASE READ THE NOTES BELOW!

# unkown
DRIVE1 /dev/nvme0n1
# unkown
DRIVE2 /dev/nvme1n1
# unkown
DRIVE3 /dev/nvme2n1

## if you dont want raid over your three drives then comment out the following line and set SWRAIDLEVEL not to 5
## please make sure the DRIVE[nr] variable is strict ascending with the used harddisks, when you comment out one or more harddisks


## ===============
##  SOFTWARE RAID:
## ===============

## activate software RAID?  < 0 | 1 >

SWRAID 1

## Choose the level for the software RAID < 0 | 1 | 5 | 10 >

SWRAIDLEVEL 5

## ==========
##  HOSTNAME:
## ==========

## which hostname should be set?
##

HOSTNAME CentOS-85-64-minimal


## ================
##  NETWORK CONFIG:
## ================

# IPV4_ONLY no


## ==========================
##  PARTITIONS / FILESYSTEMS:
## ==========================

## define your partitions and filesystems like this:
##
## PART  <mountpoint/lvm/btrfs.X>  <filesystem/VG>  <size in MB>
##
## * <mountpoint/lvm/btrfs.X>
##            mountpoint for this filesystem *OR*
##            keyword 'lvm' to use this PART as volume group (VG) for LVM *OR*
##            identifier 'btrfs.X' to use this PART as volume for
##            btrfs subvolumes. X can be replaced with a unique
##            alphanumeric keyword
##            NOTE: no support btrfs multi-device volumes
## * <filesystem/VG>
##            can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap  *OR*  name
##            of the LVM volume group (VG), if this PART is a VG.
## * <size>
##            you can use the keyword 'all' to assign all the
##            remaining space of the drive to the *last* partition.
##            you can use M/G/T for unit specification in MiB/GiB/TiB
##
## notes:
##   - extended partitions are created automatically
##   - '/boot' cannot be on a xfs filesystem
##   - '/boot' cannot be on LVM!
##   - when using software RAID 0, you need a '/boot' partition
##
## example without LVM (default):
## -> 4GB   swapspace
## -> 512MB /boot
## -> 10GB  /
## -> 5GB   /tmp
## -> all the rest to /home
#PART swap   swap        4G
#PART /boot  ext2      512M
#PART /      ext4       10G
#PART /tmp   xfs         5G
#PART /home  ext3       all
#
##
## to activate LVM, you have to define volume groups and logical volumes
##
## example with LVM:
#
## normal filesystems and volume group definitions:
## -> 512MB boot  (not on lvm)
## -> all the rest for LVM VG 'vg0'
#PART /boot  ext3     512M
#PART lvm    vg0       all
#
## logical volume definitions:
#LV <VG> <name> <mount> <filesystem> <size>
#
#LV vg0   root   /        ext4         10G
#LV vg0   swap   swap     swap          4G
#LV vg0   tmp    /tmp     reiserfs      5G
#LV vg0   home   /home    xfs          20G
#
##
## to use btrfs subvolumes, define a volume identifier on a partition
##
## example with btrfs subvolumes:
##
## -> all space on one partition with volume 'btrfs.1'
#PART btrfs.1    btrfs       all
##
## btrfs subvolume definitions:
#SUBVOL <volume> <subvolume> <mount>
#
#SUBVOL btrfs.1  @           /
#SUBVOL btrfs.1  @/usr       /usr
#SUBVOL btrfs.1  @home       /home
#
## your system has the following devices:
#
# Disk /dev/nvme0n1: 512 GB (=> 476 GiB) doesn't contain a valid partition table
# Disk /dev/nvme1n1: 512 GB (=> 476 GiB) doesn't contain a valid partition table
# Disk /dev/nvme2n1: 2048 GB (=> 1907 GiB) doesn't contain a valid partition table
#
## Based on your disks and which RAID level you will choose you have
## the following free space to allocate (in GiB):
# RAID  0: ~1428
# RAID  1: ~476
# RAID  5: ~952
#

PART swap swap 32G
PART /boot ext3 1024M
PART / ext4 all


## ========================
##  OPERATING SYSTEM IMAGE:
## ========================

## full path to the operating system image
##   supported image sources:  local dir,  ftp,  http,  nfs
##   supported image types: tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz
## examples:
#
# local: /path/to/image/filename.tar.gz
# ftp:   ftp://<user>:<password>@hostname/path/to/image/filename.tar.bz2
# http:  http://<user>:<password>@hostname/path/to/image/filename.tbz
# https: https://<user>:<password>@hostname/path/to/image/filename.tbz
# nfs:   hostname:/path/to/image/filename.tgz
#
# for validation of the image, place the detached gpg-signature
# and your public key in the same directory as your image file.
# naming examples:
#  signature:   filename.tar.bz2.sig
#  public key:  public-key.asc

IMAGE /root/.oldroot/nfs/install/../images/CentOS-85-64-minimal.tar.gz

```