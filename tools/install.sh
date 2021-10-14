#!/bin/bash

mv /usr/bin/nvidia-container-runtime-hook        /usr/bin/nvidia-container-runtime-hook-bak
mv /host/usr/bin/mount_nano_gpu                  /host/usr/bin/mount_nano_gpu-bak
cp /usr/bin/hook                                 /host/usr/bin/nvidia-container-runtime-hook
cp /usr/bin/mount_nano_gpu                       /host/usr/bin/mount_nano_gpu