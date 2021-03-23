#!/usr/bin/bash

# This script prepares testing environment on a remote s390x
# machine in beaker and runs specified tests on it over ssh.

# Run on lastest nightly
DISTRO=$(curl -L http://download.devel.redhat.com/rhel-8/nightly/RHEL-8/latest-RHEL-8.4/COMPOSE_ID)
TEST_RUN=$1

run_command () {
    ssh root@"$BEAKER_HOSTNAME" "\$1"
}

# Add beaker-client repo
cat > beaker-client.repo <<EOF
[beaker-client]
name=Beaker Client - RedHatEnterpriseLinux8
baseurl=http://download.eng.bos.redhat.com/beakerrepos/client/RedHatEnterpriseLinux8/
enabled=1
gpgcheck=0
EOF
sudo mv beaker-client.repo /etc/yum.repos.d/

# Install beaker-client
sudo dnf install -y beaker-client

# Recipe for beaker
cat > beaker.xml <<EOF
<job retention_tag="scratch">
  <whiteboard>osbuild-composer s390x testing</whiteboard>
  <recipeSet priority="High">
    <recipe whiteboard="" role="RECIPE_MEMBERS" ks_meta="" kernel_options="" kernel_options_post="">
      <autopick random="false"/>
      <watchdog panic="ignore"/>
      <packages/>
      <ks_appends/>
      <repos/>
      <distroRequires>
        <and>
          <distro_family op="=" value="RedHatEnterpriseLinux8"/>
          <distro_variant op="=" value="BaseOS"/>
          <distro_name op="=" value="$DISTRO"/>
          <distro_arch op="=" value="s390x"/>
        </and>
      </distroRequires>
      <hostRequires>
        <and>
          <hostname op="like" value="s390x-kvm-%.lab.eng.rdu2.redhat.com"/>
        </and>
      </hostRequires>
      <partitions/>
      <task name="/distribution/check-install" role="STANDALONE"/>
      <task name="/distribution/reservesys" role="STANDALONE">
        <params>
          <param name="RESERVETIME" value="7200"/>
        </params>
      </task>
    </recipe>
  </recipeSet>
</job>
EOF

# Submit the job
JOB_ID=$(bkr job-submit beaker.xml | awk -F'['\''|'\'']' '{print $2}')

# Wait for the machine to get provisioned
for i in {1..120}
do
if bkr job-results "$JOB_ID" | grep Completed > /dev/null;
then
echo "Sytem is provisioned." && break
fi
if [ "$i" -eq 120 ]
then
echo "Giving up after 2 hours." && exit 1
fi
echo "Waiting for system to get provisioned"
sleep 60
done

BEAKER_HOSTNAME=$(bkr job-results "$JOB_ID" | grep -oPz -m1 'system value=".*?\/' | awk -F'"' '{print $2}')

# Prepare the machine for running tests
run_command "dnf -y install git rpm-build"
run_command "git clone https://github.com/osbuild/osbuild-composer.git"
run_command "dnf -y builddep osbuild-composer/osbuild-composer.spec"
run_command "cd osbuild-composer && make rpm"
run_command "dnf -y install osbuild-composer/rpmbuild/RPMS/s390x/osbuild-composer*.rpm"

# Run selected tests
case $TEST_RUN in

    base)
      run_command "./osbuild-composer/test/cases/base_tests.sh"
      ;;
    image)
      export DISTRO_CODE="rhel8"
      export BUILD_ID="$JOB_ID"
      run_command "./osbuild-composer/test/cases/image_tests.sh"
      ;;
    libvirt)
      run_command "./osbuild-composer/test/cases/libvirt.sh"
      ;;
    *)
      echo "Unknown test run specified."
      exit 1
      ;;
esac

# Return the machine
run_command "return2beaker.sh"

exit 0
