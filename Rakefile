# FreeBSD Ports
#
# Douglas Thrift
#
# Rakefile

require 'date'
require 'ostruct'
require 'pathname'
require 'set'
require 'tmpdir'

def in_portdir(&block)
  portdir = Pathname.new(Rake.original_dir)
  raise "#{portdir} is not a port directory" unless portdir.relative_path_from(@portsdir).to_s =~ %r{[^/]+/[^/]+}
  Dir.chdir(portdir) do
    block.call(portdir)
  end
end

def update_portstree_map(map = {})
  `poudriere ports -l -q`.each_line do |line|
    line.chomp!
    if line =~ /^([^ ]+) +([^ ]+) +([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}) +([^ ]+)$/
      map[$1.to_sym] = OpenStruct.new(method: $2, timestamp: DateTime.parse($3), path: Pathname.new($4))
    else
      raise "unexpected line: #{line}"
    end
  end
  map
end

def with_portsdir(path, &block)
  # Set PORTSDIR around a command rather than prefixing it, so rake echoes the
  # command alone. Restored afterwards so it cannot leak into another task in
  # the same invocation (rake portlint diff).
  old = ENV['PORTSDIR']
  ENV['PORTSDIR'] = path.to_s
  block.call
ensure
  ENV['PORTSDIR'] = old
end

def clean_version(version)
  if version =~ /^(\d+\.\d+-RELEASE)(?:-p\d+)$/
    $1
  else
    version
  end
end

portstree_map = update_portstree_map
portsdir = @portsdir = Pathname.pwd
distdir = portsdir.join('distfiles')

task default: %i(update_githead)

desc 'Create or update the port checksum file (distinfo)'
task :makesum do
  in_portdir do
    sh "make makesum DEV_WARNING_WAIT=0 DISTDIR=#{distdir}"
  end
end

desc 'Regenerate the port patches in files from the edited sources'
task makepatch: :update_githead do
  in_portdir do
    # makepatch regenerates files/patch-* from .orig copies under WRKDIR. It is
    # not rake diff[patch], which produces the single diff a PR attaches; these
    # patches ship inside the port and the framework applies them at build time.
    #
    # make makepatch never fetches -- it regenerates patches from .orig copies
    # already under WRKDIR -- so unlike makesum this needs no DISTDIR. WRKDIR
    # and PATCHDIR both resolve inside this working copy whichever tree PORTSDIR
    # names. The one thing it takes from the tree is SCRIPTSDIR's
    # smart_makepatch.sh, which decides patch naming and whitespace handling --
    # so run the copy from the tree update_githead keeps current rather than
    # whatever /usr/ports happens to hold. That is why this task has a
    # prerequisite the make target does not.
    with_portsdir(githead_portstree.path) do
      sh "make makepatch"
    end
  end
end

# These are for checking a fetched distfile against the hash upstream publishes,
# not for distinfo -- CHECKSUM_ALGORITHMS is sha256 alone and makesum writes it.
# So the set tracks what projects publish: sha512 is the common one now, sha256
# and the older md5/sha1 still appear.
%w(md5 sha1 sha256 sha512).each do |sum|
  desc "Show the #{sum.upcase} sums of the distfiles"
  task sum do
    sh "mkdir -p #{distdir}" unless distdir.exist?
    files = distdir.glob('**/*').select {|dist| dist.file?}.map {|dist| dist.relative_path_from(portsdir)}
    sh "#{sum} #{files.join(' ')}" unless files.empty?
  end
end

desc 'Remove the ports distfiles'
task :distclean do
  in_portdir do
    sh "make distclean DISTDIR=#{distdir}"
  end
end

desc 'Verify the port using portlint'
task portlint: :update_githead do
  in_portdir do |portdir|
    # portlint reads MOVED, UIDs, GIDs and Mk/ out of PORTSDIR, which defaults
    # to /usr/ports -- a tree nothing here manages, and which was four days
    # behind when this was written. Point it at the githead tree this task
    # already pays a sudo to keep current, so the -C/-A decision below and the
    # lint itself consult one tree rather than two of different vintages.
    with_portsdir(githead_portstree.path) do
      if githead_portstree.path.join(portdir.relative_path_from(portsdir), 'Makefile').exist?
        sh "portlint -C"
      else
        sh "portlint -A"
      end
    end
  end
end

desc 'Check the port Makefile variable order and style using portclippy'
task :portclippy do
  in_portdir do
    sh "portclippy Makefile"
  end
end

desc 'Format the port Makefile in place using portfmt'
task :portfmt do
  in_portdir do
    sh "portfmt -i Makefile"
  end
end

desc 'Show the port differences against the ports tree'
task diff: :update_githead do |_, args|
  patch = args.extras.find {|arg| arg == 'patch'}
  tree = args.extras.find {|arg| arg != 'patch'} || 'githead'
  portstree = portstree_map[tree.to_sym] or raise "no such portstree: #{tree}"
  in_portdir do |portdir|
    rel_portdir = portdir.relative_path_from(portsdir)
    tree_portdir = portstree.path.join(rel_portdir)
    # git diff --no-index compares two directories without either being a
    # repository, so nothing is copied into the ports tree and no root is
    # needed. --no-prefix drops the leading slash from both absolute paths,
    # which are then rewritten to the a/ and b/ the Porter's Handbook expects
    # a submitted diff to carry. git repeats one path on the "diff --git" line
    # for added and deleted files, so that line is normalised separately.
    #
    # A new port has no directory in the tree, so it is diffed against an empty
    # one. /dev/null will not do: given a file and a directory, git compares
    # the file with the directory's entry of the same basename, a nonexistent
    # <port>/null.
    #
    # --no-index reads both directories straight off disk and applies no
    # exclude rules, so left to itself it would diff work directories, *.orig
    # copies and anything else .gitignore keeps out of this repo. The files are
    # therefore named as pathspecs: those git sees here, tracked or untracked
    # but not ignored, plus everything in the tree's copy so that deletions
    # still show. --literal-pathspecs keeps a file name from being read as a
    # glob.
    files = IO.popen(%w(git ls-files -z --cached --others --exclude-standard .), &:read).split("\0")
    raise "git ls-files failed with status #{$?.exitstatus}" unless $?.success?
    if tree_portdir.exist?
      files |= tree_portdir.glob('**/*', File::FNM_DOTMATCH).select(&:file?).map {|file| file.relative_path_from(tree_portdir).to_s}
    end
    diff = Dir.mktmpdir do |empty|
      IO.popen(['git', '--literal-pathspecs', 'diff', '--no-index', '--no-prefix',
                tree_portdir.exist? ? tree_portdir.to_s : empty,
                portdir.to_s, '--', *files], err: %i(child out), &:read)
    end
    # --no-index exits 1 when there are differences; above that is an error,
    # whose message would otherwise be printed, or written out, as the diff.
    raise "git diff failed with status #{$?.exitstatus}: #{diff}" if $?.exitstatus > 1
    diff = diff.gsub("#{tree_portdir.to_s[1..]}/", "a/#{rel_portdir}/")
      .gsub("#{portdir.to_s[1..]}/", "b/#{rel_portdir}/")
      .gsub(%r{^diff --git [ab]/(\S+) [ab]/(\S+)$}) {"diff --git a/#{$1} b/#{$2}"}
    if patch
      # PKGNAME depends on Mk/ -- PYTHON_PKGNAMEPREFIX and friends come from
      # there -- so resolve it against the same tree being diffed against
      # rather than whatever PORTSDIR happens to default to.
      pkgname = with_portsdir(portstree.path) {`make -V PKGNAME`.chomp}
      file = portsdir.join("#{pkgname}.diff")
      file.write(diff)
      puts "wrote #{file}"
    else
      puts diff
    end
  end
end

desc 'Test the port with poudriere'
task :testport, %i(jailname flavor) => %i(update_githead register_overlay) do |_, args|
  this_version = clean_version(`uname -r`.chomp)
  this_arch = `uname -m`.chomp
  this_jailname = nil
  `poudriere jail -l -q`.each_line do |line|
    line.chomp!
    if line =~ /^([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+).+$/
      jailname, version, arch = $1, $2, $3
      # The creation method says nothing about whether a jail is the right one to
      # test in; version and architecture do. This used to require 'ftp', which
      # stopped matching anything when poudriere's default became 'http'.
      if clean_version(version) == this_version && arch == this_arch
        this_jailname = jailname
        break
      end
    else
      raise "unexpected line: #{line}"
    end
  end
  raise "no #{this_version} #{this_arch} jail found" if this_jailname.nil? && args.jailname.nil?
  args.with_defaults(jailname: this_jailname)
  in_portdir do |portdir|
    rel_portdir = portdir.relative_path_from(portsdir)
    # -b latest prefetches binary packages for the dependencies instead of
    # building them, matching the cron bulk builds and caronade's recipe.
    sh "sudo poudriere testport -b latest -j #{args.jailname} -o #{rel_portdir}#{args.flavor && "@#{args.flavor}"} -p githead -O overlay"
  end
end

{githead: 'git+https'}.each do |name, method|
  define_method("#{name}_portstree") do
    portstree_map[name]
  end

  desc "Create or update the #{name} portstree"
  task "update_#{name}" do |_, args|
    force = args.extras.find {|arg| arg == 'force'}
    portstree = portstree_map[name]
    if portstree
      raise "unexpected portstree method for #{name}: #{portstree[:method]} (expected: #{method})" if portstree[:method] != method
      if force || portstree.timestamp < Date.today
        sh "sudo poudriere ports -u -p #{name}"
      end
    else
      sh "sudo poudriere ports -c -m #{method} -p #{name}"
      update_portstree_map(portstree_map)
    end
  end
end

desc 'Register this working copy as the overlay portstree'
task :register_overlay do
  portstree = portstree_map[:overlay]
  if portstree
    raise "unexpected overlay portstree path: #{portstree.path} (expected: #{portsdir})" if portstree.path != portsdir
  else
    # -m null registers an existing directory without poudriere creating or
    # populating it, and -f none is required or the create errors out on the
    # directory already existing. poudriere testport -O then nullfs-mounts this
    # over the base tree, so the build reads this working copy directly and
    # nothing has to be copied into the root-owned githead tree.
    sh "sudo poudriere ports -c -m null -M #{portsdir} -f none -p overlay"
    update_portstree_map(portstree_map)
  end
end
