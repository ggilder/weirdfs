#!/usr/bin/env ruby

require 'fileutils'

# Basic script to rename folders based on modification dates of files within
USAGE = <<EOD
rename_folders.rb [--dry-run] [PATH]

`--dry-run` prints the actions to be taken without performing them.
PATH is optional; defaults to current directory.
EOD

if ARGV.delete('--help') || ARGV.delete('-h')
  puts USAGE
  exit
end

dry_run = ARGV.delete('--dry-run')
path = ARGV.shift || Dir.pwd
path = File.expand_path(path)

puts "Scanning #{path} for folders without dates..."

folders = Dir[File.join(path, '*')].select do |file|
  File.directory?(file) && File.basename(file) !~ %r{^\d{4}-\d{2}-\d{2} } && File.basename(file) !~ /^~/
end

folders.each do |folder|
  mtimes = Dir[File.join(folder, '*')].map { |f| File.mtime(f) }
  mtimes.sort!
  created_time = mtimes.first
  if created_time
    new_name = File.join(File.dirname(folder), "#{created_time.strftime('%Y-%m-%d')} #{File.basename(folder)}")
    puts %(Renaming "#{File.basename(folder)}" to "#{File.basename(new_name)}".)
    unless dry_run
      FileUtils.mv(folder, new_name, verbose: true)
    end
  else
    puts "No created time found for #{folder}! Skipping."
  end
end

puts "Done."
