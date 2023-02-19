ssh -i ~/jamsynckeypair.pem ec2-user@ssh.prod.jamsync.dev << EOF
  rm -rf jamsync-build
  unzip -o jamsync-build.zip
  unzip jamsync-build/public/jamsync-source.zip -d jamsync-build
  cd jamsync-build
  make buildservers
  sudo systemctl restart jamsyncserver jamsyncweb
  exit
EOF