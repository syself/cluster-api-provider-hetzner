# CAPH pre-provision-command

Update the caph deployment:

```yaml

      # New init container, which makes the command available to caph.
      initContainers:
      - command:
        - /bin/sh
        - -c
        - cp /my-pre-provision-command.sh /shared/
        image: ghcr.io/syself/caph-staging:pre-provision-command
        imagePullPolicy: Always
        name: init-container
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /shared
          name: shared


      # Add this to the args of the caph container:
        args:
        - --pre-provision-command=/shared/my-pre-provision-command.sh

        # Add this to the caph container
        volumeMounts:
        - mountPath: /shared
          name: shared

      # Add this after "container"
      volumes:
      - emptyDir: {}
        name: shared
```
