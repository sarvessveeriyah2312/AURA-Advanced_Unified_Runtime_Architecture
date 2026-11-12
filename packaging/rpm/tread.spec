# RPM spec for the Tread SDK (ADR-0011 §7). Built by
# packaging/rpm/build-rpm.sh from a released SDK archive; it repackages
# the archive's prebuilt binary and never compiles anything.
%global debug_package %{nil}
%global __os_install_post %{nil}
%global __strip /bin/true

Name:           tread
Version:        %{tread_version}
Release:        1
Summary:        Tread programming language SDK
License:        MIT
URL:            https://treadlang.vercel.app/
AutoReqProv:    no

%description
Tread is a language for learning how languages work. This package
installs the Tread SDK (compiler, bytecode tools and runtime) in
/opt/tread and links /usr/bin/tread to it.

%install
mkdir -p %{buildroot}/opt/tread %{buildroot}/usr/bin
cp -a %{tread_src}/. %{buildroot}/opt/tread/
chmod 0755 %{buildroot}/opt/tread/bin/tread
ln -s /opt/tread/bin/tread %{buildroot}/usr/bin/tread

%files
%dir /opt/tread
%dir /opt/tread/bin
%dir /opt/tread/docs
/opt/tread/bin/tread
/opt/tread/docs/*
/opt/tread/release
/usr/bin/tread
