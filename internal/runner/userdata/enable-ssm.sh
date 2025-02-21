#!/bin/bash

if ! command -v amazon-ssm-agent; then
    RPM_URL="https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_amd64/amazon-ssm-agent.rpm"
    DEB_URL="https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/debian_amd64/amazon-ssm-agent.deb"

    if command -v dnf; then
        dnf install -y "$RPM_URL"
    elif command -v yum; then
        yum install -y "$RPM_URL"
    elif command -v zypper; then
        wget "$RPM_URL"
        rpm -i amazon-ssm-agent.rpm
    elif command -v dpkg; then
        wget "$DEB_URL"
        dpkg -i amazon-ssm-agent.deb
    fi
fi

if command -v systemctl; then
    systemctl enable amazon-ssm-agent
    systemctl start amazon-ssm-agent
fi
