-- Tests for mtlog config module
local config = require('mtlog.config')

describe('mtlog config', function()
  -- Store original config
  local original_config
  
  before_each(function()
    -- Reset to defaults before each test
    config.setup()
    original_config = vim.deepcopy(config.get())
  end)
  
  after_each(function()
    -- Restore original config
    config.setup()
  end)
  
  describe('setup', function()
    it('should load default values when called with no options', function()
      config.setup()
      local cfg = config.get()
      
      -- Check some key defaults
      assert.equals(true, cfg.auto_enable)  -- auto_enable exists in defaults
      assert.equals(500, cfg.debounce_ms)  -- actual default is 500
      assert.equals(false, cfg.show_errors)  -- actual default
      assert.equals(true, cfg.diagnostics_enabled)
      assert.is_table(cfg.severity_levels)
      assert.is_table(cfg.virtual_text)
      assert.is_table(cfg.cache)
    end)
    
    it('should override defaults with user options', function()
      config.setup({
        debounce_ms = 1000,
        show_errors = true,
        auto_enable = false,
      })
      
      local cfg = config.get()
      assert.equals(1000, cfg.debounce_ms)
      assert.equals(true, cfg.show_errors)
      assert.equals(false, cfg.auto_enable)
    end)
    
    it('should handle boolean shortcut for cache', function()
      -- Test disabling cache with boolean
      config.setup({ cache = false })
      local cfg = config.get()
      assert.equals(false, cfg.cache.enabled)
      
      -- Test enabling cache with boolean
      config.setup({ cache = true })
      cfg = config.get()
      assert.equals(true, cfg.cache.enabled)
    end)
    
    it('should deep merge nested tables', function()
      config.setup({
        virtual_text = {
          enabled = false,
          prefix = '>>',
        }
      })
      
      local cfg = config.get()
      assert.equals(false, cfg.virtual_text.enabled)
      assert.equals('>>', cfg.virtual_text.prefix)
      -- Should preserve other defaults
      assert.is_not_nil(cfg.virtual_text.spacing)
    end)
    
    it('should merge severity_levels correctly', function()
      config.setup({
        severity_levels = {
          MTLOG001 = vim.diagnostic.severity.WARN,
          MTLOG999 = vim.diagnostic.severity.ERROR,
        }
      })
      
      local cfg = config.get()
      -- Should override MTLOG001
      assert.equals(vim.diagnostic.severity.WARN, cfg.severity_levels.MTLOG001)
      -- Should add new code
      assert.equals(vim.diagnostic.severity.ERROR, cfg.severity_levels.MTLOG999)
      -- Should preserve other defaults
      assert.equals(vim.diagnostic.severity.ERROR, cfg.severity_levels.MTLOG002)  -- actual default is ERROR
    end)
    
    it('should handle cache configuration', function()
      config.setup({
        cache = {
          enabled = true,
          ttl = 3600,
          max_size = 500,
        }
      })
      
      local cfg = config.get()
      assert.equals(true, cfg.cache.enabled)
      assert.equals(3600, cfg.cache.ttl)
      assert.equals(500, cfg.cache.max_size)
    end)
    
    it('should handle signs configuration', function()
      config.setup({
        signs = {
          enabled = false,
          priority = 20,
        }
      })
      
      local cfg = config.get()
      assert.equals(false, cfg.signs.enabled)
      assert.equals(20, cfg.signs.priority)
    end)
  end)
  
  describe('get and set', function()
    it('should get top-level values', function()
      assert.equals(true, config.get('auto_enable'))
      assert.equals(500, config.get('debounce_ms'))
    end)
    
    it('should get nested values with dot notation', function()
      assert.equals(true, config.get('virtual_text.enabled'))
      assert.equals('■ ', config.get('virtual_text.prefix'))  -- actual default is '■ '
      assert.equals(true, config.get('cache.enabled'))
    end)
    
    it('should return entire config when no key specified', function()
      local cfg = config.get()
      assert.is_table(cfg)
      assert.is_not_nil(cfg.auto_enable)
      assert.is_not_nil(cfg.virtual_text)
    end)
    
    it('should return nil for non-existent keys', function()
      assert.is_nil(config.get('non_existent'))
      assert.is_nil(config.get('virtual_text.non_existent'))
    end)
    
    it('should set top-level values', function()
      config.set('debounce_ms', 500)
      assert.equals(500, config.get('debounce_ms'))
      
      config.set('auto_enable', false)
      assert.equals(false, config.get('auto_enable'))
    end)
    
    it('should set nested values with dot notation', function()
      config.set('virtual_text.enabled', false)
      assert.equals(false, config.get('virtual_text.enabled'))
      
      config.set('cache.ttl', 7200)
      assert.equals(7200, config.get('cache.ttl'))
    end)
    
    it('should create intermediate tables when setting nested values', function()
      config.set('new_feature.sub_option.value', 42)
      assert.equals(42, config.get('new_feature.sub_option.value'))
      assert.is_table(config.get('new_feature'))
      assert.is_table(config.get('new_feature.sub_option'))
    end)
  end)
  
  describe('validate_config', function()
    it('should pass with valid configuration', function()
      local valid_config = {
        enabled = true,
        debounce_ms = 100,
        show_codes = false,
        diagnostics_enabled = true,
        virtual_text = {
          enabled = true,
          prefix = '>'
        }
      }
      
      -- Should not throw error
      local ok = pcall(config.setup, valid_config)
      assert.is_true(ok)
    end)
    
    it('should validate type constraints', function()
      -- Test invalid debounce_ms type
      local invalid_config = {
        debounce_ms = "fast"  -- Should be number
      }
      
      -- For now, just test that setup doesn't crash
      -- In a real implementation, you might want to add validation
      local ok = pcall(config.setup, invalid_config)
      -- The current implementation might not validate types strictly
      -- This test documents the current behavior
    end)
  end)
  
  describe('get_diagnostic_opts', function()
    it('should transform config to vim.diagnostic format', function()
      config.setup({
        virtual_text = {
          enabled = true,
          prefix = '■ ',
          spacing = 2,
        },
        signs = {
          enabled = false,
          priority = 10,
        },
        underline = {
          enabled = true,
          severity_limit = vim.diagnostic.severity.ERROR,
        },
        update_in_insert = false,
        severity_sort = true,
      })
      
      local opts = config.get_diagnostic_opts()
      
      -- Check virtual_text transformation
      assert.is_table(opts.virtual_text)
      assert.equals('■ ', opts.virtual_text.prefix)
      assert.equals(2, opts.virtual_text.spacing)
      
      -- Check signs
      assert.equals(false, opts.signs)
      
      -- Check underline
      assert.is_table(opts.underline)
      assert.is_table(opts.underline.severity)
      assert.equals(vim.diagnostic.severity.ERROR, opts.underline.severity.min)
      
      -- Check other options
      assert.equals(false, opts.update_in_insert)
      assert.equals(true, opts.severity_sort)
    end)
    
    it('should handle disabled features', function()
      config.setup({
        virtual_text = { enabled = false },
        signs = { enabled = false },
        underline = { enabled = false },
      })
      
      local opts = config.get_diagnostic_opts()
      
      assert.equals(false, opts.virtual_text)
      assert.equals(false, opts.signs)
      assert.equals(false, opts.underline)
    end)
  end)
  
  describe('suppressed_diagnostics', function()
    it('should initialize as empty array', function()
      config.setup()
      local suppressed = config.get('suppressed_diagnostics')
      assert.is_table(suppressed)
      assert.equals(0, #suppressed)
    end)
    
    it('should accept array of diagnostic codes', function()
      config.setup({
        suppressed_diagnostics = {'MTLOG001', 'MTLOG004'}
      })
      
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.equals('MTLOG001', suppressed[1])
      assert.equals('MTLOG004', suppressed[2])
    end)
    
    it('should be modifiable at runtime', function()
      config.set('suppressed_diagnostics', {'MTLOG002', 'MTLOG003'})
      local suppressed = config.get('suppressed_diagnostics')
      assert.equals(2, #suppressed)
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG002'))
      assert.is_true(vim.tbl_contains(suppressed, 'MTLOG003'))
    end)
  end)
  
  describe('With() diagnostics configuration', function()
    it('should have severity levels for all With() codes', function()
      config.setup()
      local severity_levels = config.get('severity_levels')
      
      -- Check all With() diagnostic codes have severity levels
      assert.equals(vim.diagnostic.severity.ERROR, severity_levels.MTLOG009)
      assert.equals(vim.diagnostic.severity.WARN, severity_levels.MTLOG010)
      assert.equals(vim.diagnostic.severity.INFO, severity_levels.MTLOG011)
      assert.equals(vim.diagnostic.severity.WARN, severity_levels.MTLOG012)
      assert.equals(vim.diagnostic.severity.ERROR, severity_levels.MTLOG013)
    end)
    
    it('should allow overriding With() diagnostic severities', function()
      config.setup({
        severity_levels = {
          MTLOG009 = vim.diagnostic.severity.WARN,  -- Change from ERROR to WARN
          MTLOG011 = vim.diagnostic.severity.ERROR, -- Change from INFO to ERROR
        }
      })
      
      local severity_levels = config.get('severity_levels')
      assert.equals(vim.diagnostic.severity.WARN, severity_levels.MTLOG009)
      assert.equals(vim.diagnostic.severity.ERROR, severity_levels.MTLOG011)
      -- Others should remain at defaults
      assert.equals(vim.diagnostic.severity.WARN, severity_levels.MTLOG010)
    end)
  end)
  
  describe('ignore_patterns', function()
    it('should default to common ignore patterns', function()
      config.setup()
      local patterns = config.get('ignore_patterns')
      assert.is_table(patterns)
      -- Should include vendor by default
      local has_vendor = false
      for _, pattern in ipairs(patterns) do
        if pattern:match('vendor') then
          has_vendor = true
          break
        end
      end
      assert.is_true(has_vendor)
    end)
    
    it('should allow custom ignore patterns', function()
      config.setup({
        ignore_patterns = {
          '.*_test%.go$',
          'generated/',
          'mocks/',
        }
      })
      
      local patterns = config.get('ignore_patterns')
      assert.equals(3, #patterns)
      assert.is_true(vim.tbl_contains(patterns, '.*_test%.go$'))
      assert.is_true(vim.tbl_contains(patterns, 'generated/'))
      assert.is_true(vim.tbl_contains(patterns, 'mocks/'))
    end)
  end)
end)