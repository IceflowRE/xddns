FROM archlinux:latest

RUN pacman -Syu --noconfirm \
    base-devel \
    git \
    sudo \
    bash \
    go \
    namcap \
    pacman-contrib \
    devtools && \
    pacman -Scc --noconfirm
    
RUN pacman -Syu --noconfirm && \
    pacman -S --noconfirm systemd systemd-sysvcompat dbus && \
    pacman -Scc --noconfirm

RUN (cd /lib/systemd/system/sysinit.target.wants/ && for i in *; do [ $i == systemd-tmpfiles-setup.service ] || rm -f $i; done); \
    rm -f /lib/systemd/system/multi-user.target.wants/*;\
    rm -f /etc/systemd/system/*.wants/*;\
    rm -f /lib/systemd/system/local-fs.target.wants/*;\
    rm -f /lib/systemd/system/sockets.target.wants/*udev*;\
    rm -f /lib/systemd/system/sockets.target.wants/*initctl*;\
    rm -f /lib/systemd/system/basic.target.wants/*;

ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME && \
    useradd --uid $USER_UID --gid $USER_GID -m $USERNAME && \
    echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/$USERNAME && \
    chmod 0440 /etc/sudoers.d/$USERNAME

WORKDIR /home/$USERNAME

USER $USERNAME

USER root
    
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
