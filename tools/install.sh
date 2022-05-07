#!/bin/bash
mv /host/usr/bin/nvidia-container-runtime-hook /host/usr/bin/nvidia-container-runtime-hook-bak
mv /host/usr/bin/nvidia-container-toolkit /host/usr/bin/nvidia-container-toolkit-bak
cp --preserve=timestamps /usr/bin/egpu-nvidia-container-runtime-hook /host/usr/bin/nvidia-container-runtime-hook
cp --preserve=timestamps /usr/bin/egpu-nvidia-container-toolkit /host/usr/bin/nvidia-container-toolkit