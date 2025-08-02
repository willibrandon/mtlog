package com.mtlog.analyzer.inspection

import com.goide.inspections.core.GoInspectionBase
import com.goide.psi.GoFile
import com.intellij.codeInspection.*
import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.annotator.MtlogExternalAnnotator
import com.mtlog.analyzer.service.MtlogProjectService

class MtlogBatchInspection : LocalInspectionTool() {
    
    override fun getShortName(): String = "MtlogTemplateValidation"
    
    override fun getDisplayName(): String = MtlogBundle.message("inspection.display.name")
    
    override fun getGroupDisplayName(): String = "Go"
    
    override fun isEnabledByDefault(): Boolean = true
    
    override fun runForWholeFile(): Boolean = true
    
    override fun checkFile(
        file: PsiFile,
        manager: InspectionManager,
        isOnTheFly: Boolean
    ): Array<ProblemDescriptor> {
        if (file !is GoFile) return ProblemDescriptor.EMPTY_ARRAY
        
        val project = file.project
        val service = project.service<MtlogProjectService>()
        if (!service.state.enabled) return ProblemDescriptor.EMPTY_ARRAY
        
        val problems = mutableListOf<ProblemDescriptor>()
        
        // Use the external annotator logic for batch inspection
        val annotator = MtlogExternalAnnotator()
        val info = annotator.collectInformation(file) ?: return ProblemDescriptor.EMPTY_ARRAY
        val result = annotator.doAnnotate(info) ?: return ProblemDescriptor.EMPTY_ARRAY
        
        for (diagnostic in result.diagnostics) {
            val element = file.findElementAt(diagnostic.range.startOffset) ?: continue
            
            val severity = when (diagnostic.severity) {
                com.mtlog.analyzer.annotator.DiagnosticSeverity.ERROR -> ProblemHighlightType.ERROR
                com.mtlog.analyzer.annotator.DiagnosticSeverity.WARNING -> ProblemHighlightType.WARNING
                com.mtlog.analyzer.annotator.DiagnosticSeverity.SUGGESTION -> ProblemHighlightType.WEAK_WARNING
            }
            
            val fixes = mutableListOf<LocalQuickFix>()
            
            when {
                diagnostic.message.contains("PascalCase") && diagnostic.propertyName != null -> {
                    fixes.add(PascalCaseLocalQuickFix(diagnostic.propertyName))
                }
                diagnostic.message.contains("arguments") -> {
                    fixes.add(TemplateArgumentLocalQuickFix())
                }
            }
            
            problems.add(
                manager.createProblemDescriptor(
                    element,
                    diagnostic.message,
                    isOnTheFly,
                    fixes.toTypedArray(),
                    severity
                )
            )
        }
        
        return problems.toTypedArray()
    }
    
    // Local quick fix wrappers for batch inspection
    private class PascalCaseLocalQuickFix(
        private val propertyName: String
    ) : LocalQuickFix {
        override fun getName(): String = MtlogBundle.message("quickfix.pascalcase.name")
        override fun getFamilyName(): String = MtlogBundle.message("quickfix.family.name")
        
        override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
            val element = descriptor.psiElement ?: return
            com.mtlog.analyzer.quickfix.PascalCaseQuickFix(element, propertyName)
                .invoke(project, element.containingFile, null, element, element)
        }
    }
    
    private class TemplateArgumentLocalQuickFix : LocalQuickFix {
        override fun getName(): String = MtlogBundle.message("quickfix.template.arguments.name")
        override fun getFamilyName(): String = MtlogBundle.message("quickfix.family.name")
        
        override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
            val element = descriptor.psiElement ?: return
            com.mtlog.analyzer.quickfix.TemplateArgumentQuickFix(element)
                .invoke(project, element.containingFile, null, element, element)
        }
    }
}