# Making a new release

This guide describes the process of releasing osbuild-composer both [upstream][upstream-git] and into [Fedora][fedora-distgit] and [CentOS Stream][centos-distgit].

> This guide assumes that the number of the new release is stored in an environment variable called `VERSION`.

## Upstream release

Osbuild-composer release is a tagged commit that's merged into the `main` branch as any other contribution.

### Making a release PR

Firstly, a new branch from (up-to-date) main needs to be created:

```
git checkout main
git pull
git checkout -b release-version-$VERSION
```

All unreleased news have to copied over to a new directory:

```
mkdir docs/news/$VERSION
mv docs/news/unreleased/* docs/news/$VERSION
```

Now, the `NEWS.md` file must be updated. A template for a new release can be generated using `make release`:

```
make release
your-favourite-editor NEWS.md
```

The next step is to update the version number in the spec file. You can use `your-favourite-editor`, or this lovely `sed` script:

```
sed -i -E "s/(Version:\\s+)[0-9]+/\1$VERSION/" osbuild-composer.spec
```

At this moment, everything in the checkout is ready for making the release commit:

```
git add osbuild-composer.spec NEWS.md docs/news/unreleased docs/news/$VERSION
git commit -s -m $VERSION -m "Release osbuild-composer $VERSION"
```

As the final step, push the branch into your fork and open a PR against the `main` branch. The PR now needs to be approved and merged.

### After the release PR is merged

Firstly, make sure to update the local checkout of the `main` branch:

```
git checkout main
git pull
```

Tag the release and push the tag:

```
git tag -s -m 'osbuild-composer $VERSION' v$VERSION HEAD
git push upstream v$VERSION
```

The last thing to do is to create a new release on GitHub. The form can be found at [this webpage][upstream-draft-a-new-release]. The tag version is simply `v$VERSION`, and the release title is just `$VERSION`. The release description should contain the newly added entry from `NEWS.md`.

The upstream release process is now done, now it's time to push the newly born release into downstreams.

## Fedora release

In order to push the new release into Fedora, you need to be [a Fedora packager][new-fedora-packager] and have commit rights for [the repository][fedora-distgit]. If you don't have them, create [an issue upstream][upstream-new-issue].

Start by cloning the dist-git repository and selecting the `rawhide` branch:

```
fedpkg clone osbuild-composer
fedpkg switch-branch rawhide
```

> You should always start with updating the latest Fedora release.
> 
> Also, note that `fedpkg` is in many cases just a wrapper around `git`. You can use `git checkout` instead of `fedpkg switch-branch`, `git push` instead of `fedpkg push` etc.

Now, you need to update the sources and specfile. Luckily, we have a handy script in the repository that does the following:

- It merges the upstream spec file with changelog from the downstream spec file and adds there an entry for the new release.
- It uploads the tarball into the distgit's look-side cache.
- It commits the updated spec file and sources file.

Note that the script needs to have a valid Kerberos ticket in order to upload the tarball into the lookaside cache. To use the helper, just run:

```
$OSBUILD_COMPOSER/tools/update-distgit.py \
  --version $VERSION \
  --author "Name Surname <email@example.com>" \
  --pkgtool fedpkg
```

`$OSBUILD_COMPOSER` contains the path to your local upstream osbuild-composer checkout.

After the script is finished, it doesn't hurt to perform two checks:
- Review the commit that the helper created.
- Make a scratch build so you know that the new version is indeed buildable in Fedora.

You can do these steps by running:

```
git show HEAD
fedpkg scratch-build --srpm
```

After the scratch build successfully finishes, push the changes and make a real build:

```
fedpkg push
fedpkg build
```

After you are done with rawhide, switch to the newest stable release for Fedora and do the same change there. It's preferred to reuse the commit from `rawhide`: 

```
fedpkg switch-branch f34
git merge --ff-only rawhide
```

If a fast-forward merge it not possible, you can for example cherry-pick the latest commit from `rawhide` using `git cherry-pick rawhide`. If it doesn't apply cleanly, you need to figure out what happened in the git history. Note that you cannot force-push into dist-git. Once something is there, it cannot be removed.

Now, it's time for the scratch-build check. If it passes, we can safely push the changes into the dist-git and  make a regular build.

```
fedpkg scratch-build --srpm
fedpkg push
fedpkg build
```

For stable Fedora releases, it's also needed to create an update in Bodhi:

```
fedpkg update --type enhancement --notes "Update osbuild-composer to the latest version"
```

> Feeling lazy? Just run the following line, grab a cup of coffee and watch it do all the hard work for you:
> ```
> fedpkg scratch-build --srpm && fedpkg push && fedpkg build && fedpkg update --type enhancement --notes "Update osbuild-composer to the latest version"
> ```

After this is done, continue with the older stable release(s). After all of them are done, the work on Fedora is over. The updates will not appear immediately, it takes them a week to get through the `updates-testing` repository.

## CentOS Stream 9 release

There's a wonderful guide on this topic in the section 5 of RHEL Developer Guide. `update-distgit.py` can also save you some time here. The only differences from Fedora are that `--pkgtool centos` and `--release c9s` flags need to be used.

At the time of writing this document, gating tests are not run in CentOS dist-git. You also have to simultaneously open a PR in RHEL dist-git to verify that the test suite passes. Close this PR once the PR in CentOS dist-git is merged.

## Spreading the word on osbuild.org

The last of releasing a new version is to create a new post on osbuild.org. Just open a PR in [osbuild/osbuild.github.io]. You can find a lot of inspiration in existing release posts.

[upstream-git]: https://github.com/osbuild/osbuild-composer
[fedora-distgit]: https://src.fedoraproject.org/rpms/osbuild-composer
[centos-distgit]: https://gitlab.com/redhat/centos-stream/rpms/osbuild-composer
[upstream-draft-a-new-release]: https://github.com/osbuild/osbuild-composer/releases/new
[new-fedora-packager]: https://fedoraproject.org/wiki/Join_the_package_collection_maintainers
[upstream-new-issue]: https://github.com/osbuild/osbuild-composer/issues/new/choose
[osbuild/osbuild.github.io]: https://github.com/osbuild/osbuild.github.io
