#!/usr/bin/env ruby

require 'fileutils'

# Basic script to rename files based on their modification dates
USAGE = <<EOD
rename_files.rb [--dry-run] PATH...

`--dry-run` prints the actions to be taken without performing them.
PATH may be repeated.
EOD

if ARGV.delete('--help') || ARGV.delete('-h')
  puts USAGE
  exit
end

dry_run = ARGV.delete('--dry-run')
paths = ARGV.map { |f| File.expand_path(f) }

puts "Renaming #{paths.count} files..."
puts "DRY RUN, no renaming will take place" if dry_run

paths.each do |path|
  name = File.basename(path)
  if name =~ %r{^\d{4}-\d{2}-\d{2} }
    puts %{[INFO] File "#{name}" already appears to be dated; skipping.}
    next
  end
  mtime = File.mtime(path)
  new_name = "#{mtime.strftime('%Y-%m-%d')} #{name}"
  new_path = File.join(File.dirname(path), new_name)
  if File.exist?(new_path)
    puts %{[ERROR] Destination "#{new_path}" already exists.}
  else
    puts %(Renaming "#{name}" to "#{new_name}".)
    unless dry_run
      FileUtils.mv(path, new_path, verbose: true)
    end
  end
end

puts "Done."
